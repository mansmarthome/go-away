package lib

import (
	"errors"
	"net"
	"testing"

	"git.gammaspectra.live/git/go-away/lib/policy"
	"git.gammaspectra.live/git/go-away/utils"
	"github.com/yl2chen/cidranger"
)

type fakeASN struct {
	rec   utils.ASNRecord
	ok    bool
	err   error
	calls int
}

func (f *fakeASN) Lookup(net.IP) (utils.ASNRecord, bool, error) {
	f.calls++
	return f.rec, f.ok, f.err
}

func (f *fakeASN) Close() error { return nil }

func TestNetworkSetContains(t *testing.T) {
	_, prefix, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	ranger := cidranger.NewPCTrieRanger()
	if err := ranger.Insert(cidranger.NewBasicRangerEntry(*prefix)); err != nil {
		t.Fatal(err)
	}

	prefixOnly := &networkSet{ranger: ranger}
	ok, err := prefixOnly.Contains(net.ParseIP("10.1.2.3"))
	if err != nil || !ok {
		t.Fatalf("prefix hit: ok=%v err=%v", ok, err)
	}

	asnHit := &networkSet{
		ranger: cidranger.NewPCTrieRanger(),
		asns:   map[uint32]struct{}{64500: {}},
		db:     &fakeASN{rec: utils.ASNRecord{ASN: 64500}, ok: true},
	}
	ok, err = asnHit.Contains(net.ParseIP("45.45.45.10"))
	if err != nil || !ok {
		t.Fatalf("asn hit: ok=%v err=%v", ok, err)
	}

	asnMiss := &networkSet{
		ranger: cidranger.NewPCTrieRanger(),
		asns:   map[uint32]struct{}{64500: {}},
		db:     &fakeASN{rec: utils.ASNRecord{ASN: 1}, ok: true},
	}
	ok, err = asnMiss.Contains(net.ParseIP("45.45.45.10"))
	if err != nil || ok {
		t.Fatalf("asn miss: ok=%v err=%v", ok, err)
	}

	asnZero := &networkSet{
		ranger: cidranger.NewPCTrieRanger(),
		asns:   map[uint32]struct{}{64500: {}},
		db:     &fakeASN{rec: utils.ASNRecord{ASN: 0}, ok: true},
	}
	ok, err = asnZero.Contains(net.ParseIP("45.45.46.10"))
	if err != nil || ok {
		t.Fatalf("asn 0: ok=%v err=%v", ok, err)
	}

	asnZeroHit := &networkSet{
		ranger: cidranger.NewPCTrieRanger(),
		asns:   map[uint32]struct{}{0: {}},
		db:     &fakeASN{rec: utils.ASNRecord{ASN: 0}, ok: true},
	}
	ok, err = asnZeroHit.Contains(net.ParseIP("45.45.46.10"))
	if err != nil || !ok {
		t.Fatalf("asn 0 hit: ok=%v err=%v", ok, err)
	}

	mixed := &networkSet{
		ranger: ranger,
		asns:   map[uint32]struct{}{64500: {}},
		db:     &fakeASN{rec: utils.ASNRecord{ASN: 99999, Country: "US"}, ok: true},
	}
	ok, err = mixed.Contains(net.ParseIP("10.1.2.3"))
	if err != nil || !ok {
		t.Fatalf("mixed prefix hit: ok=%v err=%v", ok, err)
	}

	lookupErr := errors.New("lookup failed")
	broken := &networkSet{
		ranger: cidranger.NewPCTrieRanger(),
		asns:   map[uint32]struct{}{64500: {}},
		db:     &fakeASN{err: lookupErr},
	}
	_, err = broken.Contains(net.ParseIP("45.45.45.10"))
	if !errors.Is(err, lookupErr) {
		t.Fatalf("lookup error: %v", err)
	}
}

func TestNetworkSetCountry(t *testing.T) {
	db := &fakeASN{rec: utils.ASNRecord{ASN: 10075, Country: "BD"}, ok: true}
	set := &networkSet{
		ranger:    cidranger.NewPCTrieRanger(),
		asns:      map[uint32]struct{}{64500: {}},
		countries: map[string]struct{}{"BD": {}},
		db:        db,
	}
	ok, err := set.Contains(net.ParseIP("103.219.224.253"))
	if err != nil || !ok || db.calls != 1 {
		t.Fatalf("country hit: ok=%v err=%v calls=%d", ok, err, db.calls)
	}

	miss := &fakeASN{rec: utils.ASNRecord{ASN: 1, Country: "US"}, ok: true}
	set.db = miss
	ok, err = set.Contains(net.ParseIP("1.2.3.4"))
	if err != nil || ok || miss.calls != 1 {
		t.Fatalf("country miss: ok=%v err=%v calls=%d", ok, err, miss.calls)
	}

	none := &fakeASN{rec: utils.ASNRecord{ASN: 1, Country: ""}, ok: true}
	set.db = none
	ok, err = set.Contains(net.ParseIP("1.2.3.4"))
	if err != nil || ok {
		t.Fatalf("none: ok=%v err=%v", ok, err)
	}

	_, prefix, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	ranger := cidranger.NewPCTrieRanger()
	if err := ranger.Insert(cidranger.NewBasicRangerEntry(*prefix)); err != nil {
		t.Fatal(err)
	}
	prefixDB := &fakeASN{rec: utils.ASNRecord{Country: "US"}, ok: true}
	prefixHit := &networkSet{ranger: ranger, countries: map[string]struct{}{"BD": {}}, db: prefixDB}
	ok, err = prefixHit.Contains(net.ParseIP("10.1.2.3"))
	if err != nil || !ok || prefixDB.calls != 0 {
		t.Fatalf("prefix ignores country: ok=%v err=%v calls=%d", ok, err, prefixDB.calls)
	}

	asnDB := &fakeASN{rec: utils.ASNRecord{ASN: 64500, Country: "US"}, ok: true}
	asnHit := &networkSet{
		ranger:    cidranger.NewPCTrieRanger(),
		asns:      map[uint32]struct{}{64500: {}},
		countries: map[string]struct{}{"BD": {}},
		db:        asnDB,
	}
	ok, err = asnHit.Contains(net.ParseIP("45.45.45.10"))
	if err != nil || !ok || asnDB.calls != 1 {
		t.Fatalf("asn hit despite country: ok=%v err=%v calls=%d", ok, err, asnDB.calls)
	}
}

func TestValidatePolicyCountry(t *testing.T) {
	bd := "bd"
	bad := "USA"
	none := "None"
	p := policy.Policy{Networks: map[string][]policy.Network{
		"ok": {{Country: &bd}},
	}}
	if err := validatePolicyNetworks(p); err != nil {
		t.Fatal(err)
	}
	if !policyUsesASN(p) {
		t.Fatal("country entry should require the ASN database")
	}
	p.Networks["bad"] = []policy.Network{{Country: &bad}}
	if err := validatePolicyNetworks(p); err == nil {
		t.Fatal("expected invalid country")
	}
	p.Networks["bad"] = []policy.Network{{Country: &none}}
	if err := validatePolicyNetworks(p); err == nil {
		t.Fatal("expected None to be rejected")
	}
}
