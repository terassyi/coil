package model

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"regexp"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/clientv3util"
	"github.com/cybozu-go/coil"
)

var (
	poolNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]*$`)
)

// AddPool adds a new address pool.
// pool must be valid and should have only one subnet.
// name must match this regexp: ^[a-z][a-z0-9_.-]*$
func (m Model) AddPool(ctx context.Context, name string, pool *coil.AddressPool) error {
	if !poolNamePattern.MatchString(name) {
		return errors.New("invalid pool name: " + name)
	}
	err := pool.Validate()
	if err != nil {
		return err
	}
	if len(pool.Subnets) != 1 {
		return errors.New("no subnet in pool")
	}

	data, err := json.Marshal(pool)
	if err != nil {
		return err
	}

	emptyAssign := coil.EmptyAssignment(pool.Subnets[0], pool.BlockSize)
	assigns, err := json.Marshal(emptyAssign)
	if err != nil {
		return err
	}

	pkey := poolKey(name)
	skey := subnetKey(pool.Subnets[0])
	bkey := blockKey(name, pool.Subnets[0])
	resp, err := m.etcd.Txn(ctx).
		If(clientv3util.KeyMissing(pkey)).
		Then(
			clientv3.OpTxn(
				[]clientv3.Cmp{clientv3util.KeyMissing(skey)},
				[]clientv3.Op{
					clientv3.OpPut(pkey, string(data)),
					clientv3.OpPut(skey, ""),
					clientv3.OpPut(bkey, string(assigns)),
				},
				nil)).
		Commit()
	if err != nil {
		return err
	}
	if !resp.Succeeded {
		return ErrPoolExists
	}
	if !resp.Responses[0].GetResponseTxn().Succeeded {
		return ErrUsedSubnet
	}
	return nil
}

// AddSubnet adds a subnet to an existing pool.
func (m Model) AddSubnet(ctx context.Context, name string, n *net.IPNet) error {
	pkey := poolKey(name)
	skey := subnetKey(n)
	bkey := blockKey(name, n)

RETRY:
	resp, err := m.etcd.Get(ctx, pkey)
	if err != nil {
		return err
	}

	if resp.Count == 0 {
		return ErrNotFound
	}

	rev := resp.Kvs[0].ModRevision

	p := new(coil.AddressPool)
	err = json.Unmarshal(resp.Kvs[0].Value, p)
	if err != nil {
		return err
	}

	p.Subnets = append(p.Subnets, n)
	err = p.Validate()
	if err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}

	emptyAssign := coil.EmptyAssignment(n, p.BlockSize)
	assigns, err := json.Marshal(emptyAssign)
	if err != nil {
		return err
	}

	tresp, err := m.etcd.Txn(ctx).
		If(clientv3.Compare(clientv3.ModRevision(pkey), "=", rev)).
		Then(
			clientv3.OpTxn(
				[]clientv3.Cmp{clientv3util.KeyMissing(skey)},
				[]clientv3.Op{
					clientv3.OpPut(pkey, string(data)),
					clientv3.OpPut(skey, ""),
					clientv3.OpPut(bkey, string(assigns)),
				},
				nil,
			)).
		Commit()
	if err != nil {
		return err
	}
	if !tresp.Succeeded {
		goto RETRY
	}
	if !tresp.Responses[0].GetResponseTxn().Succeeded {
		return ErrUsedSubnet
	}
	return nil
}

// GetPool gets pool
func (m Model) GetPool(ctx context.Context, name string) (*coil.AddressPool, error) {
	pkey := poolKey(name)
	resp, err := m.etcd.Get(ctx, pkey)
	if err != nil {
		return nil, err
	}

	if resp.Count == 0 {
		return nil, ErrNotFound
	}
	p := new(coil.AddressPool)
	err = json.Unmarshal(resp.Kvs[0].Value, p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// RemovePool removes pool.
func (m Model) RemovePool(ctx context.Context, name string) error {
	pkey := poolKey(name)
	resp, err := m.etcd.Get(ctx, pkey)
	if err != nil {
		return err
	}

	if resp.Count == 0 {
		return ErrNotFound
	}

	p := new(coil.AddressPool)
	err = json.Unmarshal(resp.Kvs[0].Value, p)
	if err != nil {
		return err
	}

	ops := []clientv3.Op{clientv3.OpDelete(pkey)}
	for _, n := range p.Subnets {
		ops = append(ops,
			clientv3.OpDelete(subnetKey(n)),
			clientv3.OpDelete(blockKey(name, n)),
		)
	}

	_, err = m.etcd.Txn(ctx).Then(ops...).Commit()
	return err
}
