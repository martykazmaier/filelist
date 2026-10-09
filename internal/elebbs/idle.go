package elebbs

import (
	"os"
	"time"
)

// CONFIGrecord.UserTimeOut (struct.250), inactivity limit in seconds.
const configUserTimeOutOff = 1479

// ReadUserTimeOut returns the inactivity limit from CONFIG.RA. Zero means
// the sysop disabled it or CONFIG.RA could not be read.
func ReadUserTimeOut(sysPath string) time.Duration {
	p := FindFile(sysPath, "config.ra", "CONFIG.RA")
	if p == "" {
		p = findRAFile(sysPath, "config.ra", "CONFIG.RA")
	}
	if p == "" {
		return 0
	}
	data, err := os.ReadFile(p)
	if err != nil || len(data) < configUserTimeOutOff+2 {
		return 0
	}
	return time.Duration(u16(data, configUserTimeOutOff)) * time.Second
}
