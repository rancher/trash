package main

var gomoduleTemplates=`module github.com/rancher/rancher

require (
	{{- range $key, $import := .Imports}}
    {{$import.Package}} {{$import.Version}}
    {{- end}}
)

replace (
	{{- range $key, $import := .Imports}}
    {{- if (ne $import.Repo "")}}
    {{$import.Package}} {{$import.Version}} => {{$import.Repo}} {{$import.Version}}
    {{- end}}
    {{- end}}
)
`