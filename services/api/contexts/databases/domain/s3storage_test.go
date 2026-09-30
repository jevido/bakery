package domain

import "testing"

func TestS3Storage(t *testing.T) {
	in := S3Input{Name: " Garage ", Endpoint: "http://127.0.0.1:4960/", Bucket: "bakery-backups", Prefix: "/nightly/", AccessKey: "GK1", SecretKey: "s3cret"}
	s, err := NewS3Storage(in)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Garage" || s.Endpoint != "http://127.0.0.1:4960" || s.Region != DefaultRegion || s.Prefix != "nightly" || s.SecretKey != "s3cret" {
		t.Fatalf("got %+v", s)
	}
	if got := s.Key("main", 7, "a.dump"); got != "nightly/main-7/a.dump" {
		t.Fatalf("key %q", got)
	}
	s.Prefix = ""
	if got := s.Key("main", 7, "a.dump"); got != "main-7/a.dump" {
		t.Fatalf("key without prefix %q", got)
	}
	// An empty secret on update keeps the current one.
	in.SecretKey, in.Region = "", "garage"
	if err := s.Update(in); err != nil || s.SecretKey != "s3cret" || s.Region != "garage" {
		t.Fatalf("update: %+v %v", s, err)
	}

	for _, tc := range []struct {
		mod   func(*S3Input)
		field string
	}{
		{func(i *S3Input) { i.Name = "" }, "name"},
		{func(i *S3Input) { i.Endpoint = "s3.amazonaws.com" }, "endpoint"},
		{func(i *S3Input) { i.Endpoint = "ftp://x" }, "endpoint"},
		{func(i *S3Input) { i.Endpoint = "https://x/bucket" }, "endpoint"},
		{func(i *S3Input) { i.Bucket = "ab" }, "bucket"},
		{func(i *S3Input) { i.Bucket = "Bakery" }, "bucket"},
		{func(i *S3Input) { i.Bucket = "a..b" }, "bucket"},
		{func(i *S3Input) { i.Bucket = "-abc" }, "bucket"},
		{func(i *S3Input) { i.AccessKey = " " }, "access_key"},
		{func(i *S3Input) { i.SecretKey = "" }, "secret_key"},
	} {
		in := S3Input{Name: "g", Endpoint: "https://s3.example.com", Bucket: "b-1.x", AccessKey: "a", SecretKey: "s"}
		tc.mod(&in)
		if _, err := NewS3Storage(in); fieldOf(err) != tc.field {
			t.Errorf("%+v: %v, want field %s", in, err, tc.field)
		}
	}
}
