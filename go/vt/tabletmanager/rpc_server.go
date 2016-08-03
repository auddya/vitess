// Copyright 2012, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tabletmanager

import (
	"fmt"

	log "github.com/golang/glog"
	"github.com/youtube/vitess/go/tb"
	"github.com/youtube/vitess/go/vt/callinfo"
	"github.com/youtube/vitess/go/vt/topo/topoproto"
	"golang.org/x/net/context"
)

// This file contains the RPC method helpers for the tablet manager.

//
// Utility functions for RPC service
//

// rpcWrapper handles all the logic for rpc calls.
func (agent *ActionAgent) rpcWrapper(ctx context.Context, name TabletAction, args, reply interface{}, verbose bool, f func() error, lock bool) (err error) {
	defer func() {
		if x := recover(); x != nil {
			log.Errorf("TabletManager.%v(%v) on %v panic: %v\n%s", name, args, topoproto.TabletAliasString(agent.TabletAlias), x, tb.Stack(4))
			err = fmt.Errorf("caught panic during %v: %v", name, x)
		}
	}()

	from := ""
	ci, ok := callinfo.FromContext(ctx)
	if ok {
		from = ci.Text()
	}

	if lock {
		agent.actionMutex.Lock()
		defer agent.actionMutex.Unlock()
		// After we take the lock (which could take a long
		// time), we check the client is still here.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	if err = f(); err != nil {
		log.Warningf("TabletManager.%v(%v)(on %v from %v) error: %v", name, args, topoproto.TabletAliasString(agent.TabletAlias), from, err.Error())
		return fmt.Errorf("TabletManager.%v on %v error: %v", name, topoproto.TabletAliasString(agent.TabletAlias), err)
	}
	if verbose {
		log.Infof("TabletManager.%v(%v)(on %v from %v): %#v", name, args, topoproto.TabletAliasString(agent.TabletAlias), from, reply)
	}
	return
}

// RPCWrap is for read-only actions that can be executed concurrently.
// verbose is forced to false.
func (agent *ActionAgent) RPCWrap(ctx context.Context, name TabletAction, args, reply interface{}, f func() error) error {
	return agent.rpcWrapper(ctx, name, args, reply, false /*verbose*/, f,
		false /*lock*/)
}

// RPCWrapLock is for actions that should not run concurrently with each other.
func (agent *ActionAgent) RPCWrapLock(ctx context.Context, name TabletAction, args, reply interface{}, verbose bool, f func() error) error {
	return agent.rpcWrapper(ctx, name, args, reply, verbose, f,
		true /*lock*/)
}

//
// RegisterQueryService is used to delay registration of RPC servers until we have all the objects.
type RegisterQueryService func(*ActionAgent)

// RegisterQueryServices is a list of functions to call when the delayed registration is triggered.
var RegisterQueryServices []RegisterQueryService

// registerQueryService will register all the instances.
func (agent *ActionAgent) registerQueryService() {
	for _, f := range RegisterQueryServices {
		f(agent)
	}
}
