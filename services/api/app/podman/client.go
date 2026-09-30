// Package podman is a thin client for the Podman libpod REST API, covering
// only what Bakery uses. It talks to the rootless socket over a unix
// connection; the same client will work over an SSH-tunnelled socket.
//
// CLAUDE.md forbids shelling out to the podman CLI and using Podman's own
// bindings package (its dependency tree is most of Podman).
package podman

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// apiPrefix is the libpod API version Bakery speaks. Podman serves older
// versions next to its own, and v5 has everything used here.
const apiPrefix = "/v5.0.0/libpod"

// DefaultSocket is the rootless socket of the user running Bakery.
func DefaultSocket() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return filepath.Join(dir, "podman", "podman.sock")
}

type Client struct {
	http *http.Client
}

// New returns a client for the unix socket at path.
func New(socket string) *Client {
	dialer := &net.Dialer{}
	return NewDialer(func(ctx context.Context) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", socket)
	})
}

// NewDialer returns a client that reaches the API through dial, e.g. a
// socket tunnelled over SSH (see SSHConn.Podman).
func NewDialer(dial func(ctx context.Context) (net.Conn, error)) *Client {
	return &Client{http: &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dial(ctx)
		},
	}}}
}

// Error is an error the API answered with.
type Error struct {
	Status  int
	Message string
	Cause   string
}

func (e *Error) Error() string {
	return fmt.Sprintf("podman: %s (%d)", e.Message, e.Status)
}

// IsNotFound reports whether err is the API's 404.
func IsNotFound(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == http.StatusNotFound
}

// anonymousAuth is base64("{}"). Sent on pulls and builds so the Podman
// service does not use whatever registry credentials the host user has
// saved (a stale Docker Hub login makes every pull fail). Registry
// credentials Bakery manages itself replace it per pull (PullOptions).
const anonymousAuth = "e30="

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body io.Reader, contentType string) (*http.Response, error) {
	return c.doWith(ctx, method, path, query, body, contentType, nil)
}

// doWith is do with extra request headers, which win over the defaults.
func (c *Client) doWith(ctx context.Context, method, path string, query url.Values, body io.Reader, contentType string, header http.Header) (*http.Response, error) {
	u := "http://podman" + apiPrefix + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	switch path {
	case "/images/pull":
		req.Header.Set("X-Registry-Auth", anonymousAuth)
	case "/build":
		req.Header.Set("X-Registry-Config", anonymousAuth)
	}
	for k, v := range header {
		req.Header[k] = v
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("podman: %w", err)
	}
	if res.StatusCode >= 400 {
		defer res.Body.Close()
		return nil, readError(res)
	}
	return res, nil
}

func readError(res *http.Response) error {
	var body struct {
		Cause    string `json:"cause"`
		Message  string `json:"message"`
		Response int    `json:"response"`
	}
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if json.Unmarshal(raw, &body) != nil || body.Message == "" {
		body.Message = strings.TrimSpace(string(raw))
	}
	return &Error{Status: res.StatusCode, Message: body.Message, Cause: body.Cause}
}

// call sends a JSON body (if any), decodes a JSON answer into out (if any)
// and closes the response.
func (c *Client) call(ctx context.Context, method, path string, query url.Values, in, out any) error {
	var body io.Reader
	contentType := ""
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body, contentType = bytes.NewReader(raw), "application/json"
	}
	res, err := c.do(ctx, method, path, query, body, contentType)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if out == nil {
		_, _ = io.Copy(io.Discard, res.Body)
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// exists asks an .../exists endpoint: 204 yes, 404 no.
func (c *Client) exists(ctx context.Context, path string) (bool, error) {
	err := c.call(ctx, http.MethodGet, path, nil, nil, nil)
	if IsNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

func (c *Client) Ping(ctx context.Context) error {
	return c.call(ctx, http.MethodGet, "/_ping", nil, nil, nil)
}

// Version is the version of the Podman service answering, e.g. 5.8.7.
func (c *Client) Version(ctx context.Context) (string, error) {
	var out struct {
		Version string `json:"Version"`
	}
	if err := c.call(ctx, http.MethodGet, "/version", nil, nil, &out); err != nil {
		return "", err
	}
	return out.Version, nil
}

// EnsureNetwork creates the network with DNS enabled (so containers find
// each other by name) unless it exists.
func (c *Client) EnsureNetwork(ctx context.Context, name string) error {
	return c.EnsureNetworkLabeled(ctx, name, nil)
}

// EnsureNetworkLabeled is EnsureNetwork with labels on a network it
// creates; bakery.managed=true is always one of them.
func (c *Client) EnsureNetworkLabeled(ctx context.Context, name string, labels map[string]string) error {
	ok, err := c.exists(ctx, "/networks/"+url.PathEscape(name)+"/exists")
	if err != nil || ok {
		return err
	}
	all := map[string]string{"bakery.managed": "true"}
	maps.Copy(all, labels)
	err = c.call(ctx, http.MethodPost, "/networks/create", nil, map[string]any{
		"name":        name,
		"driver":      "bridge",
		"dns_enabled": true,
		"labels":      all,
	}, nil)
	var e *Error
	if errors.As(err, &e) && e.Status == http.StatusConflict {
		return nil // created by someone else in the meantime
	}
	return err
}

// RemoveNetwork removes a network; a missing one is not an error.
func (c *Client) RemoveNetwork(ctx context.Context, name string) error {
	err := c.call(ctx, http.MethodDelete, "/networks/"+url.PathEscape(name), url.Values{"force": {"true"}}, nil, nil)
	if IsNotFound(err) {
		return nil
	}
	return err
}

// ListNetworks returns the names of the networks carrying every one of the
// labels.
func (c *Client) ListNetworks(ctx context.Context, labels map[string]string) ([]string, error) {
	var out []struct {
		Name string `json:"name"`
	}
	if err := c.call(ctx, http.MethodGet, "/networks/json", url.Values{"filters": {labelFilters(labels)}}, nil, &out); err != nil {
		return nil, err
	}
	names := make([]string, len(out))
	for i, n := range out {
		names[i] = n.Name
	}
	return names, nil
}

func (c *Client) ImageExists(ctx context.Context, ref string) (bool, error) {
	return c.exists(ctx, "/images/"+url.PathEscape(ref)+"/exists")
}

// RemoveImage force-removes an image; a missing one is not an error.
func (c *Client) RemoveImage(ctx context.Context, ref string) error {
	err := c.call(ctx, http.MethodDelete, "/images/"+url.PathEscape(ref), url.Values{"force": {"true"}}, nil, nil)
	if IsNotFound(err) {
		return nil
	}
	return err
}

// PullImage pulls ref (fully qualified, e.g. docker.io/library/caddy:2)
// anonymously with TLS verified, handing progress lines to out.
func (c *Client) PullImage(ctx context.Context, ref string, out func(line string)) error {
	_, err := c.PullImageWith(ctx, ref, PullOptions{TLSVerify: true}, out)
	return err
}

// PullOptions are the registry credentials (both empty: anonymous) and
// whether the registry's TLS certificate is verified.
type PullOptions struct {
	Username  string
	Password  string
	TLSVerify bool
}

// PullImageWith pulls ref, always asking the registry for the newest image
// behind the tag, and returns the image id.
func (c *Client) PullImageWith(ctx context.Context, ref string, opts PullOptions, out func(line string)) (string, error) {
	q := url.Values{"reference": {ref}, "policy": {"always"}, "tlsVerify": {strconv.FormatBool(opts.TLSVerify)}}
	var header http.Header
	if opts.Username != "" {
		// libpod reads a single DockerAuthConfig, base64url encoded.
		raw, _ := json.Marshal(map[string]string{"username": opts.Username, "password": opts.Password})
		header = http.Header{"X-Registry-Auth": {base64.URLEncoding.EncodeToString(raw)}}
	}
	res, err := c.doWith(ctx, http.MethodPost, "/images/pull", q, nil, "", header)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	dec := json.NewDecoder(res.Body)
	id := ""
	for {
		var msg struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
			ID     string `json:"id"`
		}
		if err := dec.Decode(&msg); err == io.EOF {
			break
		} else if err != nil {
			return "", fmt.Errorf("podman: reading pull output: %w", err)
		}
		if msg.Error != "" {
			return "", &Error{Status: http.StatusInternalServerError, Message: strings.TrimSpace(msg.Error)}
		}
		if msg.ID != "" {
			id = msg.ID
		}
		emitLines(msg.Stream, out)
	}
	if id == "" {
		return "", errors.New("podman: pull ended without an image")
	}
	return id, nil
}

// TagImage adds the name repo:tag to the image ref.
func (c *Client) TagImage(ctx context.Context, ref, repo, tag string) error {
	return c.call(ctx, http.MethodPost, "/images/"+url.PathEscape(ref)+"/tag", url.Values{"repo": {repo}, "tag": {tag}}, nil, nil)
}

// ImageDigest returns ref's image by digest in the repository it was
// pulled from, e.g. docker.io/traefik/whoami@sha256:…. One image can be
// known under several repositories; the one in ref wins.
func (c *Client) ImageDigest(ctx context.Context, ref string) (string, error) {
	var out struct {
		Digest      string   `json:"Digest"`
		RepoDigests []string `json:"RepoDigests"`
	}
	if err := c.call(ctx, http.MethodGet, "/images/"+url.PathEscape(ref)+"/json", nil, nil, &out); err != nil {
		return "", err
	}
	repo := Repository(ref)
	for _, d := range out.RepoDigests {
		if strings.HasPrefix(d, repo+"@") {
			return d, nil
		}
	}
	return repo + "@" + out.Digest, nil
}

// Repository is ref without its tag or digest.
func Repository(ref string) string {
	ref, _, _ = strings.Cut(ref, "@")
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		ref = ref[:i]
	}
	return ref
}

type BuildOptions struct {
	// Tag names the image, e.g. localhost/bakery/whoami:12.
	Tag string
	// Dockerfile is the path inside the context, e.g. Dockerfile.
	Dockerfile string
	Labels     map[string]string
	// BuildArgs are handed to the Dockerfile's ARGs.
	BuildArgs map[string]string
}

// Build builds an image from a tar of the build context, handing each output
// line to out as it arrives, and returns the image id.
func (c *Client) Build(ctx context.Context, contextTar io.Reader, opts BuildOptions, out func(line string)) (string, error) {
	q := url.Values{"t": {opts.Tag}, "dockerfile": {opts.Dockerfile}, "rm": {"true"}, "layers": {"true"}}
	if len(opts.Labels) > 0 {
		raw, _ := json.Marshal(opts.Labels)
		q.Set("labels", string(raw))
	}
	if len(opts.BuildArgs) > 0 {
		raw, _ := json.Marshal(opts.BuildArgs)
		q.Set("buildargs", string(raw))
	}
	res, err := c.do(ctx, http.MethodPost, "/build", q, contextTar, "application/x-tar")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	dec := json.NewDecoder(res.Body)
	imageID := ""
	for {
		var msg struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
			Aux    struct {
				ID string `json:"ID"`
			} `json:"aux"`
		}
		if err := dec.Decode(&msg); err == io.EOF {
			break
		} else if err != nil {
			return "", fmt.Errorf("podman: reading build output: %w", err)
		}
		if msg.Error != "" {
			return "", &Error{Status: http.StatusInternalServerError, Message: strings.TrimSpace(msg.Error)}
		}
		if msg.Aux.ID != "" {
			imageID = msg.Aux.ID
		}
		// Libpod ends a successful build with the image id alone on a line.
		if id := strings.TrimSpace(msg.Stream); imageIDLine.MatchString(id) {
			imageID = id
		}
		emitLines(msg.Stream, out)
	}
	if imageID == "" {
		return "", errors.New("podman: build ended without an image")
	}
	return imageID, nil
}

var imageIDLine = regexp.MustCompile(`^[0-9a-f]{64}$`)

// emitLines splits s into lines and hands each non-empty one to out.
func emitLines(s string, out func(string)) {
	if out == nil {
		return
	}
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if line = strings.TrimRight(line, "\r"); line != "" {
			out(line)
		}
	}
}

type PortMapping struct {
	HostIP        string `json:"host_ip,omitempty"`
	HostPort      uint16 `json:"host_port"`
	ContainerPort uint16 `json:"container_port"`
	Protocol      string `json:"protocol,omitempty"`
}

type NamedVolume struct {
	Name string `json:"Name"`
	Dest string `json:"Dest"`
	// Options are mount options, e.g. "ro".
	Options []string `json:"Options,omitempty"`
}

// ContainerSpec is the part of libpod's SpecGenerator Bakery sets.
type ContainerSpec struct {
	Name       string   `json:"name"`
	Image      string   `json:"image"`
	Command    []string `json:"command,omitempty"`
	Entrypoint []string `json:"entrypoint,omitempty"`
	WorkDir    string   `json:"work_dir,omitempty"`
	User       string   `json:"user,omitempty"`
	// Expose lists ports the container listens on, protocol by port.
	Expose        map[uint16]string         `json:"expose,omitempty"`
	Env           map[string]string         `json:"env,omitempty"`
	Labels        map[string]string         `json:"labels,omitempty"`
	Networks      map[string]map[string]any `json:"networks,omitempty"`
	PortMappings  []PortMapping             `json:"portmappings,omitempty"`
	Volumes       []NamedVolume             `json:"volumes,omitempty"`
	RestartPolicy string                    `json:"restart_policy,omitempty"`
	// ResourceLimits are the container's cgroup limits; nil is unlimited.
	ResourceLimits *ResourceLimits `json:"resource_limits,omitempty"`
	// Netns is set by CreateContainer: joining named networks needs bridge
	// mode, which rootless Podman does not pick by itself.
	Netns *namespace `json:"netns,omitempty"`
}

// ResourceLimits is the part of the OCI LinuxResources Bakery sets.
type ResourceLimits struct {
	Memory *MemoryLimit `json:"memory,omitempty"`
	CPU    *CPULimit    `json:"cpu,omitempty"`
}

// MemoryLimit is in bytes.
type MemoryLimit struct {
	Limit int64 `json:"limit"`
}

// CPULimit allows Quota µs of CPU time per Period µs: 50000/100000 is half a
// core.
type CPULimit struct {
	Quota  int64  `json:"quota"`
	Period uint64 `json:"period"`
}

// Limits returns the ResourceLimits for memoryMB megabytes and cpus cores;
// zero means no limit for either, and nil when both are zero.
func Limits(memoryMB int, cpus float64) *ResourceLimits {
	if memoryMB <= 0 && cpus <= 0 {
		return nil
	}
	l := &ResourceLimits{}
	if memoryMB > 0 {
		l.Memory = &MemoryLimit{Limit: int64(memoryMB) << 20}
	}
	if cpus > 0 {
		const period = 100000
		l.CPU = &CPULimit{Quota: int64(cpus*period + 0.5), Period: period}
	}
	return l
}

type namespace struct {
	NSMode string `json:"nsmode"`
}

// OnNetwork is a Networks value that joins the named networks.
func OnNetwork(names ...string) map[string]map[string]any {
	out := make(map[string]map[string]any, len(names))
	for _, n := range names {
		out[n] = map[string]any{}
	}
	return out
}

// NetworkWithAliases is a Networks entry joining name, where other
// containers on it also find this one by each alias.
func NetworkWithAliases(name string, aliases ...string) (string, map[string]any) {
	return name, map[string]any{"aliases": aliases}
}

// CreateContainer creates (not starts) a container and returns its id.
func (c *Client) CreateContainer(ctx context.Context, spec ContainerSpec) (string, error) {
	var out struct {
		ID string `json:"Id"`
	}
	if len(spec.Networks) > 0 {
		spec.Netns = &namespace{NSMode: "bridge"}
	}
	if err := c.call(ctx, http.MethodPost, "/containers/create", nil, spec, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.call(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/start", nil, nil, nil)
}

// StopContainer stops it, killing it after timeoutSeconds.
func (c *Client) StopContainer(ctx context.Context, id string, timeoutSeconds int) error {
	q := url.Values{"timeout": {fmt.Sprint(timeoutSeconds)}}
	return c.call(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/stop", q, nil, nil)
}

// RemoveContainer force-removes it with its anonymous volumes.
func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	q := url.Values{"force": {"true"}, "v": {"true"}}
	return c.call(ctx, http.MethodDelete, "/containers/"+url.PathEscape(id), q, nil, nil)
}

// CopyInto extracts files (relative path → content) into destDir, which must
// exist in the container; parent directories of the files are created.
func (c *Client) CopyInto(ctx context.Context, container, destDir string, files map[string][]byte) error {
	archive, err := tarFiles(files)
	if err != nil {
		return err
	}
	res, err := c.do(ctx, http.MethodPut, "/containers/"+url.PathEscape(container)+"/archive", url.Values{"path": {destDir}}, archive, "application/x-tar")
	if err != nil {
		return err
	}
	return res.Body.Close()
}

// CopyFileInto streams one file of size bytes, read from r, into destDir
// (which must exist in the container) as name, mode 0644, without holding
// it in memory.
func (c *Client) CopyFileInto(ctx context.Context, container, destDir, name string, size int64, r io.Reader) error {
	pr, pw := io.Pipe()
	go func() {
		tw := tar.NewWriter(pw)
		err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: size, Typeflag: tar.TypeReg})
		if err == nil {
			_, err = io.CopyN(tw, r, size)
		}
		if err == nil {
			err = tw.Close()
		}
		pw.CloseWithError(err)
	}()
	res, err := c.do(ctx, http.MethodPut, "/containers/"+url.PathEscape(container)+"/archive", url.Values{"path": {destDir}}, pr, "application/x-tar")
	// Unblocks the writer if the request ended before reading everything.
	pr.CloseWithError(errors.New("podman: copy ended"))
	if err != nil {
		return err
	}
	return res.Body.Close()
}

// ReadFile returns one regular file from the container.
func (c *Client) ReadFile(ctx context.Context, container, file string) ([]byte, error) {
	res, err := c.do(ctx, http.MethodGet, "/containers/"+url.PathEscape(container)+"/archive", url.Values{"path": {file}}, nil, "")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	tr := tar.NewReader(res.Body)
	for {
		hdr, err := tr.Next()
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", file, err)
		}
		if hdr.Typeflag == tar.TypeReg {
			return io.ReadAll(io.LimitReader(tr, 16<<20))
		}
	}
}

// CreateVolume creates a named volume with the labels; one that already
// exists is left as it is.
func (c *Client) CreateVolume(ctx context.Context, name string, labels map[string]string) error {
	if ok, err := c.exists(ctx, "/volumes/"+url.PathEscape(name)+"/exists"); err != nil || ok {
		return err
	}
	err := c.call(ctx, http.MethodPost, "/volumes/create", nil, map[string]any{"Name": name, "Label": labels}, nil)
	var e *Error
	if errors.As(err, &e) && e.Status == http.StatusConflict {
		return nil
	}
	return err
}

// VolumeMountpoint returns where a named volume's data lives on the host.
func (c *Client) VolumeMountpoint(ctx context.Context, name string) (string, error) {
	var out struct {
		Mountpoint string `json:"Mountpoint"`
	}
	if err := c.call(ctx, http.MethodGet, "/volumes/"+url.PathEscape(name)+"/json", nil, nil, &out); err != nil {
		return "", err
	}
	return out.Mountpoint, nil
}

type VolumeSummary struct {
	Name   string            `json:"Name"`
	Labels map[string]string `json:"Labels"`
}

// ListVolumes lists the volumes carrying every one of the labels.
func (c *Client) ListVolumes(ctx context.Context, labels map[string]string) ([]VolumeSummary, error) {
	var out []VolumeSummary
	err := c.call(ctx, http.MethodGet, "/volumes/json", url.Values{"filters": {labelFilters(labels)}}, nil, &out)
	return out, err
}

func labelFilters(labels map[string]string) string {
	filters := map[string][]string{}
	for k, v := range labels {
		filters["label"] = append(filters["label"], k+"="+v)
	}
	raw, _ := json.Marshal(filters)
	return string(raw)
}

// RemoveVolume force-removes a named volume; a missing one is not an error.
func (c *Client) RemoveVolume(ctx context.Context, name string) error {
	err := c.call(ctx, http.MethodDelete, "/volumes/"+url.PathEscape(name), url.Values{"force": {"true"}}, nil, nil)
	if IsNotFound(err) {
		return nil
	}
	return err
}

type ContainerState struct {
	Status   string `json:"Status"`
	Running  bool   `json:"Running"`
	ExitCode int    `json:"ExitCode"`
	Error    string `json:"Error"`
}

type ContainerInfo struct {
	ID     string         `json:"Id"`
	Name   string         `json:"Name"`
	Image  string         `json:"ImageName"`
	State  ContainerState `json:"State"`
	Config struct {
		Env    []string          `json:"Env"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	// PortBindings maps "<port>/<proto>" inside the container to host
	// bindings, as created (set before the container starts).
	HostConfig struct {
		PortBindings map[string][]HostBinding `json:"PortBindings"`
		Memory       int64                    `json:"Memory"`
		CPUQuota     int64                    `json:"CpuQuota"`
	} `json:"HostConfig"`
	Mounts []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
}

type HostBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

func (c *Client) InspectContainer(ctx context.Context, id string) (ContainerInfo, error) {
	var out ContainerInfo
	err := c.call(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/json", nil, nil, &out)
	return out, err
}

type ContainerSummary struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	State  string            `json:"State"`
	Labels map[string]string `json:"Labels"`
}

// ListContainers lists all containers (running or not) carrying every one
// of the labels.
func (c *Client) ListContainers(ctx context.Context, labels map[string]string) ([]ContainerSummary, error) {
	var out []ContainerSummary
	err := c.call(ctx, http.MethodGet, "/containers/json", url.Values{"all": {"true"}, "filters": {labelFilters(labels)}}, nil, &out)
	return out, err
}

// maxExecOutput bounds what Exec keeps of a command's output.
const maxExecOutput = 4 << 10

// Exec runs cmd inside a running container and returns its exit code and up
// to 4 KiB of its combined stdout and stderr. ctx bounds the whole run.
func (c *Client) Exec(ctx context.Context, container string, cmd []string) (int, string, error) {
	var output strings.Builder
	code, err := c.exec(ctx, container, cmd, func(body io.Reader) error {
		return demux(body, func(_, line string) {
			if output.Len()+len(line) < maxExecOutput {
				output.WriteString(line)
				output.WriteByte('\n')
			}
		})
	})
	if err != nil {
		return 0, "", err
	}
	return code, strings.TrimRight(output.String(), "\n"), nil
}

// ExecStream runs cmd inside a running container, writing its stdout
// unchanged into stdout (binary safe, no size limit) and returning its exit
// code and up to 4 KiB of its stderr. ctx bounds the whole run.
func (c *Client) ExecStream(ctx context.Context, container string, cmd []string, stdout io.Writer) (int, string, error) {
	stderr := &capped{max: maxExecOutput}
	code, err := c.exec(ctx, container, cmd, func(body io.Reader) error {
		return demuxTo(body, stdout, stderr)
	})
	if err != nil {
		return 0, "", err
	}
	return code, strings.TrimRight(stderr.String(), "\n"), nil
}

// exec creates and starts an exec session, hands its multiplexed output to
// read and returns the exit code once read is done.
func (c *Client) exec(ctx context.Context, container string, cmd []string, read func(io.Reader) error) (int, error) {
	var created struct {
		ID string `json:"Id"`
	}
	if err := c.call(ctx, http.MethodPost, "/containers/"+url.PathEscape(container)+"/exec", nil, map[string]any{
		"Cmd": cmd, "AttachStdout": true, "AttachStderr": true,
	}, &created); err != nil {
		return 0, err
	}
	raw, _ := json.Marshal(map[string]any{"Detach": false, "Tty": false})
	res, err := c.do(ctx, http.MethodPost, "/exec/"+url.PathEscape(created.ID)+"/start", nil, bytes.NewReader(raw), "application/json")
	if err != nil {
		return 0, err
	}
	err = read(res.Body)
	res.Body.Close()
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if err != nil {
		return 0, fmt.Errorf("podman: reading exec output: %w", err)
	}
	var info struct {
		ExitCode int  `json:"ExitCode"`
		Running  bool `json:"Running"`
	}
	if err := c.call(ctx, http.MethodGet, "/exec/"+url.PathEscape(created.ID)+"/json", nil, nil, &info); err != nil {
		return 0, err
	}
	return info.ExitCode, nil
}

// capped keeps the first max bytes written to it and drops the rest.
type capped struct {
	strings.Builder
	max int
}

func (w *capped) Write(p []byte) (int, error) {
	if room := w.max - w.Len(); room > 0 {
		w.Builder.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

// Logs hands a container's output to out line by line ("stdout" or
// "stderr"). With follow it returns when ctx ends or the container stops.
func (c *Client) Logs(ctx context.Context, id string, follow bool, tail int, out func(stream, line string)) error {
	q := url.Values{"stdout": {"true"}, "stderr": {"true"}, "follow": {fmt.Sprint(follow)}}
	if tail >= 0 {
		q.Set("tail", fmt.Sprint(tail))
	}
	res, err := c.do(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/logs", q, nil, "")
	if err != nil {
		return err
	}
	defer res.Body.Close()
	err = demux(res.Body, out)
	if ctx.Err() != nil {
		return nil
	}
	return err
}
