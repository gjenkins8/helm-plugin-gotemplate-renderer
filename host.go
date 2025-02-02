package main

type extismPointer uint64

//go:wasmimport extism:host/user kubernetes_resource_lookup
func extismKubernetesResourceLookup(apiVersion extismPointer, kind extismPointer, namespace extismPointer, name extismPointer) extismPointer

//go:wasmimport extism:host/user resolve_hostname
func extismResolveHostname(hostname extismPointer) extismPointer
