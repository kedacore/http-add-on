package main

import "github.com/magefile/mage/sh"

func deleteNS(ns string) error {
	return sh.RunV("kubectl", "delete", "namespace", ns)
}
