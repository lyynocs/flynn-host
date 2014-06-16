package main

import (
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/flynn/flynn-host/containerinit"
	lt "github.com/flynn/flynn-host/libvirt"
	"github.com/flynn/flynn-host/pinkerton"
	"github.com/flynn/flynn-host/types"
	"github.com/godbus/dbus"
)

type LibvirtLXCBackend struct {
	InitPath string
	c        *libvirt.VirConnection
}

const initdaemon = "/foo/bar/init"

func (l *LibvirtLXCBackend) Run(job *host.Job) error {
	if err := pinkerton.Pull(job.Artifact.URL); err != nil {
		return err
	}
	imageID, err := pinkerton.ImageID(job.Artifact.URL)
	if err != nil {
		return err
	}
	path, err := pinkerton.Checkout(job.ID, imageID)
	if err != nil {
		return err
	}

	if err := bindMount(l.InitPath, "/.containerinit", false, true); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(path, ".container-shared"), 0700); err != nil {
		return err
	}

	domain := &lt.Domain{
		Type:   "lxc",
		Name:   job.ID,
		Memory: lt.UnitInt{Value: 1, Unit: "GiB"},
		VCPU:   1,
		OS: lt.OS{
			Type: lt.OSType{Value: "exe"},
			Init: "/.containerinit",
		},
		Devices: lt.Devices{
			Filesystems: []lt.Filesystem{{
				Type:   "mount",
				Source: path,
				Target: "/",
			}},
		},
	}

	vd, err := l.c.DomaineDefineXML(string(domain.XML()))
	if err != nil {
		return err
	}
	if err := vd.Create(); err != nil {
		return err
	}

	go l.watchContainer(job.ID, path)

	return nil
}

func (l *LibvirtLXCBackend) watchContainer(id, path string) error {
	// We can't connect to the socket file directly because
	// the path to it is longer than 108 characters (UNIX_PATH_MAX).
	// Create a temporary symlink to connect to.
	symlink := "/tmp/containerinit-dbus." + id
	os.Symlink(path.Join(path, containerinit.SharedPath, containerinit.DBusSocketName), symlink)
	defer os.Remove(symlink)

	var err error
	var client *containerinit.Client
	for startTime := time.Now(); time.Since(startTime) < time.Second; time.Sleep(time.Millisecond) {
		var conn *dbus.Conn
		conn, err = dbus.Dial("unix:path=" + symlink)
		if err != nil {
			continue
		}
		client = containerinit.NewClient(conn)
		if err := conn.Auth(nil); err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	// hook up logging

	for exitCode := -1; exitCode == -1; {
		state, errStr, exit := client.WaitForStateChange()
		switch state {
		case containerinit.Initial:
		case containerinit.Running:
		case containerinit.Exited:
		case containerinit.FailedToStart:
		}
	}
	// emit event
	// wait for state change
	// resume
	// emit event
}

func (l *LibvirtLXCBackend) Stop(id string) error {
	// send signal to daemon
	// fallback to kill api
	// cleanup image
	return nil
}

func (l *LibvirtLXCBackend) Attach(req *AttachRequest) error {
	// attach via daemon
	return nil
}

func (l *LibvirtLXCBackend) Cleanup() error {
	// list libvirt domains
	// kill domains that are not in use
	return nil
}

func bindMount(src, dest string, writeable, private bool) error {
	srcStat, err := os.Stat(src)
	if err != nil {
		return err
	}
	destStat, err := os.Stat(dest)
	if os.IsNotExist(err) {
		if srcStat.Dir() {
			if err := os.MkdirAll(dest, 0755); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(path, os.O_CREATE, 0755)
			if err != nil {
				return err
			}
			f.Close()
		}
	} else if err != nil {
		return err
	}

	flags := syscall.MS_BIND | syscall.MS_REC
	if !writeable {
		flags |= syscall.MS_RDONLY
	}

	if err := syscall.Mount(src, dest, "bind", uintptr(flags), ""); err != nil {
		return err
	}
	if private {
		if err := syscall.Mount("", dest, "none", uintptr(syscall.MS_PRIVATE), ""); err != nil {
			return err
		}
	}
	return nil
}
