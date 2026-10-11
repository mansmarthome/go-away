package utils

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
)

func TestASNDatabaseLookup(t *testing.T) {
	path := writeASNDatabase(t, map[string]uint32{
		"1.1.1.1/32":    13335,
		"8.8.8.8/32":    15169,
		"45.45.45.0/24": 64500,
		"45.45.46.0/24": 0,
	})

	db, err := OpenASNDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	rec, ok, err := db.Lookup(net.ParseIP("45.45.45.10"))
	if err != nil || !ok || rec.ASN != 64500 || rec.Country != "" {
		t.Fatalf("hit: rec=%+v ok=%v err=%v", rec, ok, err)
	}

	mapped := net.ParseIP("45.45.45.10").To4().To16()
	rec, ok, err = db.Lookup(mapped)
	if err != nil || !ok || rec.ASN != 64500 {
		t.Fatalf("mapped: rec=%+v ok=%v err=%v", rec, ok, err)
	}

	_, ok, err = db.Lookup(net.ParseIP("9.9.9.9"))
	if err != nil || ok {
		t.Fatalf("miss: ok=%v err=%v", ok, err)
	}

	rec, ok, err = db.Lookup(net.ParseIP("45.45.46.10"))
	if err != nil || !ok || rec.ASN != 0 {
		t.Fatalf("asn 0: rec=%+v ok=%v err=%v", rec, ok, err)
	}
}

func TestASNDatabaseCountry(t *testing.T) {
	path := writeASNDatabaseRecords(t, []asnFixture{
		{cidr: "1.1.1.1/32", asn: 13335, country: "AU"},
		{cidr: "8.8.8.8/32", asn: 15169, country: "US"},
		{cidr: "45.45.45.0/24", asn: 10075, country: "BD"},
		{cidr: "45.45.46.0/24", asn: 10075, country: "None"},
		{cidr: "45.45.47.0/24", asn: 10075, country: "", hasCC: true},
		{cidr: "45.45.48.0/24", asn: 10075},
	})
	db, err := OpenASNDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	rec, ok, err := db.Lookup(net.ParseIP("45.45.45.10"))
	if err != nil || !ok || rec.Country != "BD" {
		t.Fatalf("country: rec=%+v ok=%v err=%v", rec, ok, err)
	}
	for _, ip := range []string{"45.45.46.10", "45.45.47.10", "45.45.48.10"} {
		rec, ok, err = db.Lookup(net.ParseIP(ip))
		if err != nil || !ok || rec.Country != "" {
			t.Fatalf("%s: rec=%+v ok=%v err=%v", ip, rec, ok, err)
		}
	}
}

func TestOpenASNDatabaseRejectsMissingField(t *testing.T) {
	path := writeASNDatabase(t, map[string]uint32{
		"45.45.45.0/24": 64500,
	})
	if _, err := OpenASNDatabase(path); err == nil {
		t.Fatal("expected sanity check failure")
	}
}

type asnFixture struct {
	cidr    string
	asn     uint32
	country string
	hasCC   bool
}

func writeASNDatabase(t *testing.T, records map[string]uint32) string {
	t.Helper()
	fixtures := make([]asnFixture, 0, len(records))
	for cidr, asn := range records {
		fixtures = append(fixtures, asnFixture{cidr: cidr, asn: asn})
	}
	return writeASNDatabaseRecords(t, fixtures)
}

func writeASNDatabaseRecords(t *testing.T, records []asnFixture) string {
	t.Helper()
	writer, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            "iptoasn",
		RecordSize:              24,
		IPVersion:               6,
		DisableIPv4Aliasing:     true,
		IncludeReservedNetworks: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range records {
		_, network, err := net.ParseCIDR(rec.cidr)
		if err != nil {
			t.Fatal(err)
		}
		data := mmdbtype.Map{
			"autonomous_system_number": mmdbtype.Uint32(rec.asn),
		}
		if rec.country != "" || rec.hasCC {
			data["country"] = mmdbtype.Map{"iso_code": mmdbtype.String(rec.country)}
		}
		if err := writer.Insert(network, data); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "iptoasn.mmdb")
	fh, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	if _, err := writer.WriteTo(fh); err != nil {
		t.Fatal(err)
	}
	return path
}
