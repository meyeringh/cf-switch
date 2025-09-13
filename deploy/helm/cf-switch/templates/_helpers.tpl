{{/*
Expand the name of the chart.
*/}}
{{- define "cf-switch.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cf-switch.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "cf-switch.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cf-switch.labels" -}}
helm.sh/chart: {{ include "cf-switch.chart" . }}
{{ include "cf-switch.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cf-switch.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cf-switch.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- with .Values.podSelectorLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "cf-switch.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "cf-switch.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create environment variables from values
*/}}
{{- define "cf-switch.env" -}}
{{- range $key, $value := .Values.env }}
- name: {{ $key }}
  {{- if $value.value }}
  value: {{ $value.value | quote }}
  {{- else if $value.valueFrom }}
  valueFrom:
    {{- toYaml $value.valueFrom | nindent 4 }}
  {{- end }}
{{- end }}
- name: KUBERNETES_NAMESPACE
  valueFrom:
    fieldRef:
      fieldPath: metadata.namespace
- name: KUBERNETES_SERVICE_ACCOUNT
  value: {{ include "cf-switch.serviceAccountName" . }}
{{- end }}
