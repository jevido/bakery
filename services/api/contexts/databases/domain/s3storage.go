package domain

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// S3Storage is an S3-compatible bucket Backup executions are uploaded to.
type S3Storage struct {
	ID        uint64
	Name      string
	Endpoint  string
	Region    string
	Bucket    string
	Prefix    string
	AccessKey string
	// SecretKey is never shown after it was set.
	SecretKey string
}

// S3Input is what the Owner types; an empty SecretKey on update keeps the
// current one.
type S3Input struct {
	Name, Endpoint, Region, Bucket, Prefix, AccessKey, SecretKey string
}

// DefaultRegion is the region used when none is given.
const DefaultRegion = "us-east-1"

var bucketName = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

// NewS3Storage validates the input.
func NewS3Storage(in S3Input) (S3Storage, error) {
	var s S3Storage
	return s, s.Update(in)
}

// Update validates and applies the input.
func (s *S3Storage) Update(in S3Input) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Endpoint = strings.TrimRight(strings.TrimSpace(in.Endpoint), "/")
	in.Region = strings.TrimSpace(in.Region)
	in.Bucket = strings.TrimSpace(in.Bucket)
	in.Prefix = strings.Trim(strings.TrimSpace(in.Prefix), "/")
	in.AccessKey = strings.TrimSpace(in.AccessKey)
	in.SecretKey = strings.TrimSpace(in.SecretKey)
	if in.Region == "" {
		in.Region = DefaultRegion
	}
	u, err := url.Parse(in.Endpoint)
	switch {
	case in.Name == "":
		return invalid("name", "name is required")
	case len(in.Name) > 100:
		return invalid("name", "name is at most 100 characters")
	case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "":
		return invalid("endpoint", "endpoint is an http:// or https:// URL without a path, e.g. https://s3.eu-west-1.amazonaws.com")
	case !bucketName.MatchString(in.Bucket) || strings.Contains(in.Bucket, ".."):
		return invalid("bucket", "bucket names are 3–63 lowercase letters, digits, '-' and '.'")
	case len(in.Prefix) > 200:
		return invalid("prefix", "prefix is at most 200 characters")
	case in.AccessKey == "":
		return invalid("access_key", "access key is required")
	case in.SecretKey == "" && s.SecretKey == "":
		return invalid("secret_key", "secret key is required")
	}
	s.Name, s.Endpoint, s.Region, s.Bucket, s.Prefix, s.AccessKey = in.Name, in.Endpoint, in.Region, in.Bucket, in.Prefix, in.AccessKey
	if in.SecretKey != "" {
		s.SecretKey = in.SecretKey
	}
	return nil
}

// Dir is the key prefix of one Database's Backup executions, ending in "/".
func (s S3Storage) Dir(slug string, databaseID uint64) string {
	dir := slug + "-" + strconv.FormatUint(databaseID, 10) + "/"
	if s.Prefix != "" {
		return s.Prefix + "/" + dir
	}
	return dir
}

// Key is the object key of one Backup execution file.
func (s S3Storage) Key(slug string, databaseID uint64, file string) string {
	return s.Dir(slug, databaseID) + file
}
