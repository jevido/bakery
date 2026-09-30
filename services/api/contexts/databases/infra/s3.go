package infra

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

// S3 is a thin client for one S3 storage: path-style URLs
// (<endpoint>/<bucket>/<key>) signed with Signature V4, which AWS, Garage
// and other S3-compatible stores all accept. Only what Backups need; no
// SDK, for the same reason as the Podman client.
type S3 struct {
	Storage domain.S3Storage
	HTTP    *http.Client
	// Now is the signing clock; nil is time.Now.
	Now func() time.Time
}

// maxPut is the largest object one PUT may upload.
const maxPut = 5 << 30

// emptySHA256 is the hash of an empty payload.
const emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// S3Error is an error response of the S3 API.
type S3Error struct {
	Status  int
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

func (e *S3Error) Error() string {
	switch {
	case e.Code != "" && e.Message != "":
		return fmt.Sprintf("s3: %s: %s", e.Code, e.Message)
	case e.Code != "":
		return "s3: " + e.Code
	}
	return fmt.Sprintf("s3: HTTP %d", e.Status)
}

// IsNoSuchKey reports whether err says the object does not exist.
func IsNoSuchKey(err error) bool {
	var e *S3Error
	return errors.As(err, &e) && (e.Code == "NoSuchKey" || e.Status == http.StatusNotFound)
}

func (c S3) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c S3) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// objectURL is the path-style URL of key in the bucket; key "" is the
// bucket itself.
func (c S3) objectURL(key string, query url.Values) (*url.URL, error) {
	u, err := url.Parse(c.Storage.Endpoint)
	if err != nil {
		return nil, err
	}
	u.Path = "/" + c.Storage.Bucket
	if key != "" {
		u.Path += "/" + key
	}
	u.RawPath = uriEncode(u.Path, false)
	u.RawQuery = canonicalQuery(query)
	return u, nil
}

func (c S3) request(ctx context.Context, method, key string, query url.Values, body io.Reader, size int64, payloadHash string) (*http.Response, error) {
	u, err := c.objectURL(key, query)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.ContentLength = size
	}
	sign(req, c.Storage.AccessKey, c.Storage.SecretKey, c.Storage.Region, payloadHash, c.now())
	res, err := c.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("s3: %w", err)
	}
	if res.StatusCode >= 300 {
		defer res.Body.Close()
		e := &S3Error{Status: res.StatusCode}
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		_ = xml.Unmarshal(raw, e)
		return nil, e
	}
	return res, nil
}

// Check reaches the bucket with the storage's keys (HeadBucket).
func (c S3) Check(ctx context.Context) error {
	res, err := c.request(ctx, http.MethodHead, "", nil, nil, 0, emptySHA256)
	if err != nil {
		var e *S3Error
		if errors.As(err, &e) && e.Code == "" {
			// HEAD has no body to say why; name the usual reasons.
			switch e.Status {
			case http.StatusNotFound:
				return fmt.Errorf("s3: bucket %q does not exist", c.Storage.Bucket)
			case http.StatusForbidden, http.StatusUnauthorized:
				return errors.New("s3: access denied: check the access key, the secret key and the bucket's permissions")
			case http.StatusBadRequest:
				return fmt.Errorf("s3: bad request: check the region (%s) and the endpoint", c.Storage.Region)
			}
		}
		return err
	}
	return res.Body.Close()
}

// Put uploads size bytes read from r as key, signed with their SHA-256
// (r is read twice: once to hash, once to send).
func (c S3) Put(ctx context.Context, key string, r io.ReaderAt, size int64) error {
	if size > maxPut {
		return fmt.Errorf("s3: %d bytes is more than one upload may hold (5 GiB)", size)
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.NewSectionReader(r, 0, size)); err != nil {
		return err
	}
	res, err := c.request(ctx, http.MethodPut, key, nil, io.NewSectionReader(r, 0, size), size, hex.EncodeToString(h.Sum(nil)))
	if err != nil {
		return err
	}
	return res.Body.Close()
}

// Get streams the object; the caller closes it.
func (c S3) Get(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	res, err := c.request(ctx, http.MethodGet, key, nil, nil, 0, emptySHA256)
	if err != nil {
		return nil, 0, err
	}
	return res.Body, res.ContentLength, nil
}

// Delete removes the object; a missing object is not an error.
func (c S3) Delete(ctx context.Context, key string) error {
	res, err := c.request(ctx, http.MethodDelete, key, nil, nil, 0, emptySHA256)
	if err != nil {
		if IsNoSuchKey(err) {
			return nil
		}
		return err
	}
	return res.Body.Close()
}

// List returns the keys below prefix (ListObjectsV2, every page).
func (c S3) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	token := ""
	for {
		q := url.Values{"list-type": {"2"}, "prefix": {prefix}}
		if token != "" {
			q.Set("continuation-token", token)
		}
		res, err := c.request(ctx, http.MethodGet, "", q, nil, 0, emptySHA256)
		if err != nil {
			return nil, err
		}
		var page struct {
			Contents []struct {
				Key string `xml:"Key"`
			} `xml:"Contents"`
			IsTruncated           bool   `xml:"IsTruncated"`
			NextContinuationToken string `xml:"NextContinuationToken"`
		}
		err = xml.NewDecoder(res.Body).Decode(&page)
		res.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("s3: reading the object list: %w", err)
		}
		for _, o := range page.Contents {
			keys = append(keys, o.Key)
		}
		if !page.IsTruncated || page.NextContinuationToken == "" {
			return keys, nil
		}
		token = page.NextContinuationToken
	}
}

// sign adds Signature V4 headers to req: every header already on it is
// signed, with host, x-amz-date and x-amz-content-sha256.
func sign(req *http.Request, accessKey, secretKey, region, payloadHash string, now time.Time) {
	now = now.UTC()
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)

	headers := map[string]string{"host": req.URL.Host}
	for name, values := range req.Header {
		headers[strings.ToLower(name)] = strings.TrimSpace(strings.Join(values, ","))
	}
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	var canonicalHeaders strings.Builder
	for _, name := range names {
		canonicalHeaders.WriteString(name + ":" + headers[name] + "\n")
	}
	signedHeaders := strings.Join(names, ";")

	path := req.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	canonical := strings.Join([]string{
		req.Method, path, canonicalQuery(req.URL.Query()),
		canonicalHeaders.String(), signedHeaders, payloadHash,
	}, "\n")
	scope := date + "/" + region + "/s3/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])

	key := hmacSHA256([]byte("AWS4"+secretKey), date)
	key = hmacSHA256(key, region)
	key = hmacSHA256(key, "s3")
	key = hmacSHA256(key, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(key, toSign))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+accessKey+"/"+scope+
		", SignedHeaders="+signedHeaders+", Signature="+signature)
}

func hmacSHA256(key []byte, s string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(s))
	return m.Sum(nil)
}

// canonicalQuery sorts and encodes the query as Signature V4 wants it,
// which is also a valid query string.
func canonicalQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		vs := append([]string(nil), q[k]...)
		sort.Strings(vs)
		for _, v := range vs {
			parts = append(parts, uriEncode(k, true)+"="+uriEncode(v, true))
		}
	}
	return strings.Join(parts, "&")
}

// uriEncode percent-encodes everything but RFC 3986 unreserved characters,
// and "/" unless encodeSlash.
func uriEncode(s string, encodeSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9', c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		case c == '/' && !encodeSlash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
