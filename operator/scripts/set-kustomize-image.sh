#!/bin/sh

cd config/manager
kustomize edit set image controller=$1
