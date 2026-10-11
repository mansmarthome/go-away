package lib

import (
	"errors"
	"net"
	"testing"

	"github.com/yl2chen/cidranger"
)

type fakeASN struct {
	asn uint32
	ok  bool
	err error
}

func (f fakeASN) Lookup(net.IP) (uint32, bool, error) {
	return f.asn, f.ok, f.err
}

func (f fakeASN) Close() error { return nil }

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
		db:     fakeASN{asn: 64500, ok: true},
	}
	ok, err = asnHit.Contains(net.ParseIP("45.45.45.10"))
	if err != nil || !ok {
		t.Fatalf("asn hit: ok=%v err=%v", ok, err)
	}

	asnMiss := &networkSet{
		ranger: cidranger.NewPCTrieRanger(),
		asns:   map[uint32]struct{}{64500: {}},
		db:     fakeASN{asn: 1, ok: true},
	}
	ok, err = asnMiss.Contains(net.ParseIP("45.45.45.10"))
	if err != nil || ok {
		t.Fatalf("asn miss: ok=%v err=%v", ok, err)
	}

	asnZero := &networkSet{
		ranger: cidranger.NewPCTrieRanger(),
		asns:   map[uint32]struct{}{64500: {}},
		db:     fakeASN{asn: 0, ok: true},
	}
	ok, err = asnZero.Contains(net.ParseIP("45.45.46.10"))
	if err != nil || ok {
		t.Fatalf("asn 0: ok=%v err=%v", ok, err)
	}

	mixed := &networkSet{
		ranger: ranger,
		asns:   map[uint32]struct{}{64500: {}},
		db:     fakeASN{asn: 99999, ok: true},
	}
	ok, err = mixed.Contains(net.ParseIP("10.1.2.3"))
	if err != nil || !ok {
		t.Fatalf("mixed prefix hit: ok=%v err=%v", ok, err)
	}

	lookupErr := errors.New("lookup failed")
	broken := &networkSet{
		ranger: cidranger.NewPCTrieRanger(),
		asns:   map[uint32]struct{}{64500: {}},
		db:     fakeASN{err: lookupErr},
	}
	_, err = broken.Contains(net.ParseIP("45.45.45.10"))
	if !errors.Is(err, lookupErr) {
		t.Fatalf("lookup error: %v", err)
	}
}
