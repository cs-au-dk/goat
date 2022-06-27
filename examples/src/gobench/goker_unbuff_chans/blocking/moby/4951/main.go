/*
 * Project: moby
 * Issue or PR  : https://github.com/moby/moby/pull/4951
 * Buggy version: 81f148be566ab2b17810ad4be61a5d8beac8330f
 * fix commit-id: 2ffef1b7eb618162673c6ffabccb9ca57c7dfce3
 * Flaky: 100/100
 * Description:
 *   The root cause and patch is clearly explained in the commit
 * description. The global lock is devices.Lock(), and the device
 * lock is baseInfo.lock.Lock(). It is very likely that this bug
 * can be reproduced.
 */
package main

import (
	"time"
)

type DeviceSet struct {
	mu               chan bool
	infos            map[string]*DevInfo
	nrDeletedDevices int
}

func (devices *DeviceSet) DeleteDevice(hash string) {
	devices.mu <- true
	defer func() { <-devices.mu }()

	info := devices.lookupDevice(hash)

	info.lock <- true
	defer func() { <-info.lock }()

	devices.deleteDevice(info)
}

func (devices *DeviceSet) lookupDevice(hash string) *DevInfo {
	existing, ok := devices.infos[hash]
	if !ok {
		return nil
	}
	return existing
}

func (devices *DeviceSet) deleteDevice(info *DevInfo) {
	devices.removeDeviceAndWait(info.Name())
}

func (devices *DeviceSet) removeDeviceAndWait(devname string) {
	/// remove devices by devname
	<-devices.mu
	time.Sleep(300 * time.Nanosecond)
	devices.mu <- true
}

type DevInfo struct {
	lock chan bool
	name string
}

func (info *DevInfo) Name() string {
	return info.name
}

func NewDeviceSet() *DeviceSet {
	devices := &DeviceSet{
		infos: make(map[string]*DevInfo),
		mu: func() (lock chan bool) {
			lock = make(chan bool)
			go func() {
				for {
					<-lock
					lock <- false
				}
			}()
			return
		}(),
	}
	info1 := &DevInfo{
		name: "info1",
		lock: func() (lock chan bool) {
			lock = make(chan bool)
			go func() {
				for {
					<-lock
					lock <- false
				}
			}()
			return
		}(),
	}
	info2 := &DevInfo{
		name: "info2",
		lock: func() (lock chan bool) {
			lock = make(chan bool)
			go func() {
				for {
					<-lock
					lock <- false
				}
			}()
			return
		}(),
	}
	devices.infos[info1.name] = info1
	devices.infos[info2.name] = info2
	return devices
}

func main() {

	ds := NewDeviceSet()
	/// Delete devices by the same info
	go ds.DeleteDevice("info1")
	go ds.DeleteDevice("info1")
}
