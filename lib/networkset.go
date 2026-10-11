package lib

import (
	"git.gammaspectra.live/git/go-away/utils"
	"github.com/yl2chen/cidranger"
	"net"
)

type networkSet struct {
	ranger cidranger.Ranger
	asns   map[uint32]struct{}
	db     utils.ASNLookup
}

func (n *networkSet) Contains(ip net.IP) (bool, error) {
	if n.ranger != nil && n.ranger.Len() > 0 {
		ok, err := n.ranger.Contains(ip)
		if err != nil || ok {
			return ok, err
		}
	}
	if len(n.asns) == 0 || n.db == nil {
		return false, nil
	}
	asn, ok, err := n.db.Lookup(ip)
	if err != nil {
		return false, err
	}
	if !ok || asn == 0 {
		return false, nil
	}
	_, hit := n.asns[asn]
	return hit, nil
}
