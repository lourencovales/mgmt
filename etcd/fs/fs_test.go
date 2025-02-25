// Mgmt
// Copyright (C) James Shubin and the project contributors
// Written by James Shubin <james@shubin.ca> and the project contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// Additional permission under GNU GPL version 3 section 7
//
// If you modify this program, or any covered work, by linking or combining it
// with embedded mcl code and modules (and that the embedded mcl code and
// modules which link with this program, contain a copy of their source code in
// the authoritative form) containing parts covered by the terms of any other
// license, the licensors of this program grant you additional permission to
// convey the resulting work. Furthermore, the licensors of this program grant
// the original author, James Shubin, additional permission to update this
// additional permission if he deems it necessary to achieve the goals of this
// additional permission.

//go:build !root

package fs_test // named this way to make it easier for examples

import (
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"testing"

	"github.com/purpleidea/mgmt/etcd/client"
	etcdfs "github.com/purpleidea/mgmt/etcd/fs"
	"github.com/purpleidea/mgmt/integration"
	"github.com/purpleidea/mgmt/util"
	"github.com/purpleidea/mgmt/util/errwrap"

	"github.com/spf13/afero"
)

// XXX: write a bunch more tests to test this

// TODO: apparently using 0666 is equivalent to respecting the current umask
const (
	umask      = 0666
	superblock = "/some/superblock" // TODO: generate randomly per test?
	ns         = "/_mgmt/test"      // must not end with a slash!
)

// Ensure that etcdfs.Fs implements afero.Fs.
var _ afero.Fs = &etcdfs.Fs{}

// runEtcd starts etcd locally via the mgmt binary. It returns a function to
// kill the process which the caller must use to clean up.
func runEtcd() (func() error, error) {
	// Run mgmt as etcd backend to ensure that we are testing against the
	// appropriate vendored version of etcd rather than some unknown version.
	cmdName, err := integration.BinaryPath()
	if err != nil {
		return nil, errwrap.Wrapf(err, "error getting binary path")
	}
	cmd := exec.Command(cmdName, "run", "--tmp-prefix", "empty") // empty GAPI
	if err := cmd.Start(); err != nil {
		return nil, errwrap.Wrapf(err, "error starting command %v", cmd)
	}

	return func() error {
		// cleanup when we're done
		if err := cmd.Process.Signal(syscall.SIGQUIT); err != nil {
			fmt.Printf("error sending quit signal: %+v\n", err)
		}
		if err := cmd.Process.Kill(); err != nil {
			return errwrap.Wrapf(err, "error killing process")
		}
		return nil
	}, nil
}

func TestFs1(t *testing.T) {
	stopEtcd, err := runEtcd()
	if err != nil {
		t.Errorf("setup error: %+v", err)
	}
	defer stopEtcd() // ignore the error

	logf := func(format string, v ...interface{}) {
		t.Logf("test: etcd: fs: "+format, v...)
	}
	etcdClient := client.NewClientFromSeedsNamespace(
		[]string{"localhost:2379"}, // endpoints
		ns,
	)

	if err := etcdClient.Init(); err != nil {
		t.Errorf("client connection error: %+v", err)
		return
	}
	defer etcdClient.Close()

	etcdFs := &etcdfs.Fs{
		Client:     etcdClient,
		Metadata:   superblock,
		DataPrefix: etcdfs.DefaultDataPrefix,

		Logf: logf,
	}
	//var etcdFs afero.Fs = NewEtcdFs()

	if err := etcdFs.Mkdir("/", umask); err != nil {
		t.Logf("mkdir error: %+v", err)
		if err != etcdfs.ErrExist {
			t.Errorf("mkdir error: %+v", err)
			return
		}
	}

	if err := etcdFs.Mkdir("/tmp", umask); err != nil {
		t.Errorf("mkdir2 error: %+v", err)
		return
	}

	fi, err := etcdFs.Stat("/tmp")
	if err != nil {
		t.Errorf("stat error: %+v", err)
		return
	}

	t.Logf("fi: %+v", fi)
	t.Logf("isdir: %t", fi.IsDir())

	f, err := etcdFs.Create("/tmp/foo")
	if err != nil {
		t.Errorf("create error: %+v", err)
		return
	}

	t.Logf("handle: %+v", f)

	i, err := f.WriteString("hello world!\n")
	if err != nil {
		t.Errorf("writestring error: %+v", err)
		return
	}
	t.Logf("wrote: %d", i)

	if err := etcdFs.Mkdir("/tmp/d1", umask); err != nil {
		t.Errorf("mkdir3 error: %+v", err)
		return
	}

	if err := etcdFs.Rename("/tmp/foo", "/tmp/bar"); err != nil {
		t.Errorf("rename error: %+v", err)
		return
	}

	f2, err := etcdFs.Create("/tmp/bar")
	if err != nil {
		t.Errorf("create2 error: %+v", err)
		return
	}

	i2, err := f2.WriteString("hello bar!\n")
	if err != nil {
		t.Errorf("writestring2 error: %+v", err)
		return
	}
	t.Logf("wrote: %d", i2)

	dir, err := etcdFs.Open("/tmp")
	if err != nil {
		t.Errorf("open error: %+v", err)
		return
	}
	names, err := dir.Readdirnames(-1)
	if err != nil && err != io.EOF {
		t.Errorf("readdirnames error: %+v", err)
		return
	}
	for _, name := range names {
		t.Logf("name in /tmp: %+v", name)
		return
	}

	dir, err = etcdFs.Open("/")
	if err != nil {
		t.Errorf("open2 error: %+v", err)
		return
	}
	names, err = dir.Readdirnames(-1)
	if err != nil && err != io.EOF {
		t.Errorf("readdirnames2 error: %+v", err)
		return
	}
	for _, name := range names {
		t.Logf("name in /: %+v", name)
	}
}

func TestFs2(t *testing.T) {
	stopEtcd, err := runEtcd()
	if err != nil {
		t.Errorf("setup error: %+v", err)
	}
	defer stopEtcd() // ignore the error

	logf := func(format string, v ...interface{}) {
		t.Logf("test: etcd: fs: "+format, v...)
	}
	etcdClient := client.NewClientFromSeedsNamespace(
		[]string{"localhost:2379"}, // endpoints
		ns,
	)

	if err := etcdClient.Init(); err != nil {
		t.Errorf("client connection error: %+v", err)
		return
	}
	defer etcdClient.Close()

	etcdFs := &etcdfs.Fs{
		Client:     etcdClient,
		Metadata:   superblock,
		DataPrefix: etcdfs.DefaultDataPrefix,

		Logf: logf,
	}

	tree, err := util.FsTree(etcdFs, "/")
	if err != nil {
		t.Errorf("tree error: %+v", err)
		return
	}
	t.Logf("tree: \n%s", tree)

	var memFs = afero.NewMemMapFs()

	if err := util.CopyFs(etcdFs, memFs, "/", "/", false, false); err != nil {
		t.Errorf("copyfs error: %+v", err)
		return
	}
	if err := util.CopyFs(etcdFs, memFs, "/", "/", true, false); err != nil {
		t.Errorf("copyfs2 error: %+v", err)
		return
	}
	if err := util.CopyFs(etcdFs, memFs, "/", "/tmp/d1/", false, false); err != nil {
		t.Errorf("copyfs3 error: %+v", err)
		return
	}

	tree2, err := util.FsTree(memFs, "/")
	if err != nil {
		t.Errorf("tree2 error: %+v", err)
		return
	}
	t.Logf("tree2: \n%s", tree2)
}

func TestFs3(t *testing.T) {
	stopEtcd, err := runEtcd()
	if err != nil {
		t.Errorf("setup error: %+v", err)
	}
	defer stopEtcd() // ignore the error

	logf := func(format string, v ...interface{}) {
		t.Logf("test: etcd: fs: "+format, v...)
	}
	etcdClient := client.NewClientFromSeedsNamespace(
		[]string{"localhost:2379"}, // endpoints
		ns,
	)

	if err := etcdClient.Init(); err != nil {
		t.Errorf("client connection error: %+v", err)
		return
	}
	defer etcdClient.Close()

	etcdFs := &etcdfs.Fs{
		Client:     etcdClient,
		Metadata:   superblock,
		DataPrefix: etcdfs.DefaultDataPrefix,

		Logf: logf,
	}

	if err := etcdFs.Mkdir("/tmp", umask); err != nil {
		t.Errorf("mkdir error: %+v", err)
	}
	if err := etcdFs.Mkdir("/tmp/foo", umask); err != nil {
		t.Errorf("mkdir2 error: %+v", err)
	}
	if err := etcdFs.Mkdir("/tmp/foo/bar", umask); err != nil {
		t.Errorf("mkdir3 error: %+v", err)
	}

	tree, err := util.FsTree(etcdFs, "/")
	if err != nil {
		t.Errorf("tree error: %+v", err)
		return
	}
	t.Logf("tree: \n%s", tree)

	var memFs = afero.NewMemMapFs()

	if err := util.CopyFs(etcdFs, memFs, "/tmp/foo/bar", "/", false, false); err != nil {
		t.Errorf("copyfs error: %+v", err)
		return
	}
	if err := util.CopyFs(etcdFs, memFs, "/tmp/foo/bar", "/baz/", false, false); err != nil {
		t.Errorf("copyfs2 error: %+v", err)
		return
	}

	tree2, err := util.FsTree(memFs, "/")
	if err != nil {
		t.Errorf("tree2 error: %+v", err)
		return
	}
	t.Logf("tree2: \n%s", tree2)

	if _, err := memFs.Stat("/bar"); err != nil {
		t.Errorf("stat error: %+v", err)
		return
	}
	if _, err := memFs.Stat("/baz/bar"); err != nil {
		t.Errorf("stat2 error: %+v", err)
		return
	}
}

func TestEtcdCopyFs0(t *testing.T) {
	tests := []struct {
		mkdir, cpsrc, cpdst, check string
		force                      bool
	}{
		{
			mkdir: "/",
			cpsrc: "/",
			cpdst: "/",
			check: "/",
			force: false,
		},
		{
			mkdir: "/",
			cpsrc: "/",
			cpdst: "/",
			check: "/",
			force: true,
		},
		{
			mkdir: "/",
			cpsrc: "/",
			cpdst: "/tmp/d1",
			check: "/tmp/d1",
			force: false,
		},
		{
			mkdir: "/tmp/foo/bar",
			cpsrc: "/tmp/foo/bar",
			cpdst: "/",
			check: "/bar",
			force: false,
		},
		{
			mkdir: "/tmp/foo/bar",
			cpsrc: "/tmp/foo/bar",
			cpdst: "/baz/",
			check: "/baz/bar",
			force: false,
		},
		{
			mkdir: "/tmp/foo/bar",
			cpsrc: "/tmp/foo",
			cpdst: "/baz/",
			check: "/baz/foo/bar",
			force: false,
		},
		{
			mkdir: "/tmp/this/is/a/really/deep/directory/to/make/sure/we/can/handle/deep/copies",
			cpsrc: "/tmp/this/is/a",
			cpdst: "/that/was/",
			check: "/that/was/a/really/deep/directory/to/make/sure/we/can/handle/deep/copies",
			force: false,
		},
	}

	for _, tt := range tests {
		stopEtcd, err := runEtcd()
		if err != nil {
			t.Errorf("setup error: %+v", err)
			return
		}
		defer stopEtcd() // ignore the error

		etcdClient := client.NewClientFromSeedsNamespace(
			[]string{"localhost:2379"}, // endpoints
			ns,
		)

		if err := etcdClient.Init(); err != nil {
			t.Errorf("client connection error: %+v", err)
			return
		}
		defer etcdClient.Close()

		etcdFs := &etcdfs.Fs{
			Client:     etcdClient,
			Metadata:   superblock,
			DataPrefix: etcdfs.DefaultDataPrefix,
		}

		if err := etcdFs.MkdirAll(tt.mkdir, umask); err != nil {
			t.Errorf("mkdir error: %+v", err)
			return
		}
		tree, err := util.FsTree(etcdFs, "/")
		if err != nil {
			t.Errorf("tree error: %+v", err)
			return
		}
		t.Logf("tree: \n%s", tree)

		var memFs = afero.NewMemMapFs()
		if err := util.CopyFs(etcdFs, memFs, tt.cpsrc, tt.cpdst, tt.force, false); err != nil {
			t.Errorf("copyfs error: %+v", err)
			return
		}
		tree2, err := util.FsTree(memFs, "/")
		if err != nil {
			t.Errorf("tree2 error: %+v", err)
			return
		}
		t.Logf("tree2: \n%s", tree2)
		if _, err := memFs.Stat(tt.check); err != nil {
			t.Errorf("stat error: %+v", err)
			return
		}
	}
}
