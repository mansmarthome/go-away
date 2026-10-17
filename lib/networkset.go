package lib

import (
	"git.gammaspectra.live/git/go-away/utils"
	"github.com/yl2chen/cidranger"
	"net"
)

type networkSet struct {
	ranger    cidranger.Ranger
	asns      map[uint32]struct{}
	countries map[string]struct{}
	db        utils.ASNLookup
}

func (n *networkSet) Contains(ip net.IP) (bool, error) {
	if n.ranger != nil && n.ranger.Len() > 0 {
		ok, err := n.ranger.Contains(ip)
		if err != nil || ok {
			return ok, err
		}
	}
	if n.db == nil || (len(n.asns) == 0 && len(n.countries) == 0) {
		return false, nil
	}
	rec, ok, err := n.db.Lookup(ip)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if _, hit := n.asns[rec.ASN]; hit {
		return true, nil
	}
	if rec.Country != "" {
		if _, hit := n.countries[rec.Country]; hit {
			return true, nil
		}
	}
	return false, nil
}
