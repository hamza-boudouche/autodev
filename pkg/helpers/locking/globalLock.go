package locking

import (
	"context"
	"fmt"
	"math"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func AcquireLock(client *clientv3.Client, lock string) (*clientv3.LeaseID, func() error, error) {
	var leaseID *clientv3.LeaseID
	var err error
	var releaseFunc func() error
	backoffTime := 0.5
	maxBackoffTime := 4.0
	for {
		leaseID, releaseFunc, err = acquireLockOnce(client, lock)
		if err != nil {
			fmt.Println("retrying to acquire lock ...")
			time.Sleep(time.Duration(math.Min(backoffTime, maxBackoffTime)) * time.Second)
			backoffTime *= 2
			if backoffTime > 8*maxBackoffTime {
				return nil, nil, fmt.Errorf("backoff limit, failed to acquire lock")
			}
		} else {
			break
		}
	}
	return leaseID, releaseFunc, nil
}

func acquireLockOnce(client *clientv3.Client, lock string) (*clientv3.LeaseID, func() error, error) {
	leaseResp, err := client.Grant(context.Background(), 10) // Set TTL as needed
	if err != nil {
		return nil, nil, err
	}

	txn := client.Txn(context.Background())
	txnResp, err := txn.If(
		clientv3.Compare(clientv3.CreateRevision(lock), "=", 0), // Check if key doesn't exist
	).Then(
		clientv3.OpPut(lock, "1", clientv3.WithLease(leaseResp.ID)),
	).Commit()

	if err != nil {
		return nil, nil, err
	}

	if !txnResp.Succeeded {
		// The key already exists, indicating another process holds the lock
		return nil, nil, fmt.Errorf("lock %s is already held by another process", lock)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go renewLease(ctx, client, leaseResp.ID)
	releaseFunc := func() error {
		cancel()
		return releaseLock(client, leaseResp.ID)
	}

	return &leaseResp.ID, releaseFunc, nil
}

func renewLease(ctx context.Context, client *clientv3.Client, leaseID clientv3.LeaseID) error {
	for {
		select {
		case <-ctx.Done():
			//stop renewing the lock
			return nil
		default:
			// continue renewing lock
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := client.KeepAliveOnce(ctx, leaseID)
			cancel()

			if err != nil {
				return err
			}
			fmt.Println("renewing lock ...")

			// Sleep for a shorter duration than the lease TTL to ensure timely renewals
			time.Sleep(8 * time.Second) // Adjust as needed
		}
	}
}

func releaseLock(client *clientv3.Client, leaseID clientv3.LeaseID) error {
	_, err := client.Revoke(context.Background(), leaseID)
	return err
}
