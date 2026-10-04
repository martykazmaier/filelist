//go:build !windows

package elebbs

func diskLongName(string, string, uint32) string { return "" }
