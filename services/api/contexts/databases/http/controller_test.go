package http

import (
	"encoding/json"
	"testing"

	"github.com/jevido/bakery/services/api/contexts/databases/app"
	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

func TestDatabaseRequestReadsDescriptionAndImage(t *testing.T) {
	var req databaseRequest
	body := `{"name":"db","description":"orders","version":"17","image":"postgres:16-alpine","public_port":null}`
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	in := req.input()
	if in.Description != "orders" || in.Image != "postgres:16-alpine" || in.Version != "17" || in.PublicPort != 0 {
		t.Fatalf("input %+v", in)
	}
}

func TestJSONShowsDescriptionAndImage(t *testing.T) {
	v := app.View{Database: domain.Database{Name: "db", Description: "orders", Type: domain.PostgreSQL, Version: "16-alpine"}}
	out := toJSON(v, false)
	if out.Description != "orders" || out.Image != "postgres:16-alpine" || out.Version != "16-alpine" {
		t.Fatalf("json %+v", out)
	}
}

func TestJSONShowsContainerAndVolume(t *testing.T) {
	v := app.View{Database: domain.Database{ID: 7, Slug: "orders-x1", Type: domain.MySQL, Version: "8.4"}}
	out := toJSON(v, false)
	if out.Container != "bakery-db-orders-x1" || out.Volume.Name != "bakery-db-7-data" || out.Volume.MountPath != "/var/lib/mysql" {
		t.Fatalf("json %+v", out)
	}
}
