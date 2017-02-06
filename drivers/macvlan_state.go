package drivers

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/osl"
	"github.com/docker/libnetwork/types"
)

func (d *Driver) addNetwork(n *network) {
	d.Lock()
	d.Networks[n.id] = n
	d.Unlock()
}

func (d *Driver) deleteNetwork(nid string) {
	d.Lock()
	delete(d.Networks, nid)
	d.Unlock()
}

// getNetworks Safely returns a slice of existing Networks
func (d *Driver) getNetworks() []*network {
	d.Lock()
	defer d.Unlock()

	ls := make([]*network, 0, len(d.Networks))
	for _, nw := range d.Networks {
		ls = append(ls, nw)
	}

	return ls
}

func (n *network) endpoint(eid string) *endpoint {
	n.Lock()
	defer n.Unlock()

	return n.endpoints[eid]
}

func (n *network) addEndpoint(ep *endpoint) {
	n.Lock()
	n.endpoints[ep.id] = ep
	n.Unlock()
}

func (n *network) deleteEndpoint(eid string) {
	n.Lock()
	delete(n.endpoints, eid)
	n.Unlock()
}

func (n *network) getEndpoint(eid string) (*endpoint, error) {
	n.Lock()
	defer n.Unlock()
	if eid == "" {
		return nil, fmt.Errorf("endpoint id %s not found", eid)
	}
	if ep, ok := n.endpoints[eid]; ok {
		return ep, nil
	}

	return nil, nil
}

func validateID(nid, eid string) error {
	if nid == "" {
		return fmt.Errorf("invalid network id")
	}
	if eid == "" {
		return fmt.Errorf("invalid endpoint id")
	}
	return nil
}

func (n *network) sandbox() osl.Sandbox {
	n.Lock()
	defer n.Unlock()

	return n.sbox
}

func (n *network) setSandbox(sbox osl.Sandbox) {
	n.Lock()
	n.sbox = sbox
	n.Unlock()
}

func (d *Driver) getNetwork(id string) (*network, error) {
	d.Lock()
	defer d.Unlock()
	if id == "" {
		return nil, types.BadRequestErrorf("invalid network id: %s", id)
	}
	if nw, ok := d.Networks[id]; ok {
		return nw, nil
	}

	return nil, types.NotFoundErrorf("network not found: %s", id)
}

func (d *Driver) network(nid string) *network {
	d.Lock()
	n, ok := d.Networks[nid]
	d.Unlock()
	if !ok {
		n = d.getNetworkFromSwarm(nid)
		if n != nil {
			d.Lock()
			d.Networks[nid] = n
			d.Unlock()
		}
	}

	return n
}

func (d *Driver) getNetworkFromSwarm(nid string) *network {
	if d.Client == nil {
		return nil
	}
	nw, err := d.Client.NetworkInfo(nid)
	logrus.Debugf("Network (%s)  found from swarm", nw)
	if err != nil {
		return nil
	}
	subnets := nw.IPAM.Config
	opts := nw.Options
	options := make(map[string]interface{})
	options[netlabel.GenericData] = opts
	// parse and validate the config and bind to networkConfiguration
	config, err := parseNetworkOptions(nid, options)
	if err != nil {
		return nil
	}
	if err := config.processIPAMFromSwarm(nid, subnets); err != nil {
		return nil
	}

	n := &network{
		id:        nid,
		driver:    d,
		endpoints: endpointTable{},
		config:    config,
	}

	logrus.Debugf("restore Network (%s) from swarm", n)
	return n
}