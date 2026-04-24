package main

const bomReactRev = "20260424-2"
const bomReactPath = "/bom-react"

func bomReactURL() string {
	return bomReactPath + "?rev=" + bomReactRev
}
