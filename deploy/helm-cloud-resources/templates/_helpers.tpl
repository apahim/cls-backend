{{/*
Expand the name of the chart.
*/}}
{{- define "cls-backend-cloud-resources.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "cls-backend-cloud-resources.fullname" -}}
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
{{- define "cls-backend-cloud-resources.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cls-backend-cloud-resources.labels" -}}
helm.sh/chart: {{ include "cls-backend-cloud-resources.chart" . }}
{{ include "cls-backend-cloud-resources.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cls-backend-cloud-resources.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cls-backend-cloud-resources.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Cross-chart parameter validation for consistency with application and API gateway charts
*/}}
{{- define "cls-backend-cloud-resources.validateCrossChartParams" -}}
{{- /* Validate required parameters */ -}}
{{- if not .Values.gcp.project -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: gcp.project\n\n📋 REQUIRED ACTION:\n   Set gcp.project to your Google Cloud Project ID\n\n💡 EXAMPLE:\n   gcp:\n     project: \"my-gcp-project-123\"\n\n⚠️  IMPORTANT: This value must match exactly in all three charts:\n   - helm-cloud-resources/values.yaml\n   - helm-application/values.yaml  \n   - helm-api-gateway/values.yaml\n\n🔗 More info: https://cloud.google.com/resource-manager/docs/creating-managing-projects" -}}
{{- end -}}

{{- /* Validate database configuration consistency */ -}}
{{- if not .Values.database.instance.name -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: database.instance.name\n\n📋 REQUIRED ACTION:\n   Set database.instance.name to your Cloud SQL instance name\n\n💡 EXAMPLE:\n   database:\n     instance:\n       name: \"cls-backend-db\"\n\n⚠️  IMPORTANT: This value must match database.instanceName in the application chart\n\n🔗 More info: Must be a valid Cloud SQL instance name (lowercase, hyphens allowed)" -}}
{{- end -}}

{{- if not .Values.database.database.name -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: database.database.name\n\n📋 REQUIRED ACTION:\n   Set database.database.name to your PostgreSQL database name\n\n💡 EXAMPLE:\n   database:\n     database:\n       name: \"cls_backend\"\n\n⚠️  IMPORTANT: This value must match database.databaseName in the application chart\n\n🔗 More info: Must be a valid PostgreSQL database name (lowercase, underscores allowed)" -}}
{{- end -}}

{{- if not .Values.database.user.name -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: database.user.name\n\n📋 REQUIRED ACTION:\n   Set database.user.name to your PostgreSQL username\n\n💡 EXAMPLE:\n   database:\n     user:\n       name: \"cls_user\"\n\n⚠️  IMPORTANT: This value must match database.username in the application chart\n\n🔗 More info: Must be a valid PostgreSQL username (lowercase, underscores allowed)" -}}
{{- end -}}

{{- if not .Values.database.user.passwordSecret.name -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: database.user.passwordSecret.name\n\n📋 REQUIRED ACTION:\n   Set database.user.passwordSecret.name for ESO password generation\n\n💡 EXAMPLE:\n   database:\n     user:\n       passwordSecret:\n         name: \"cls-backend-db-password\"\n\n⚠️  IMPORTANT: This value must match database.passwordSecret.name in the application chart\n\n🔗 More info: Password will be generated automatically by External Secrets Operator (ESO)" -}}
{{- end -}}

{{- /* Validate Pub/Sub configuration consistency */ -}}
{{- if not .Values.pubsub.clusterEventsTopic -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: pubsub.clusterEventsTopic\n\n📋 REQUIRED ACTION:\n   Set pubsub.clusterEventsTopic to your Pub/Sub topic name\n\n💡 EXAMPLE:\n   pubsub:\n     clusterEventsTopic: \"cluster-events\"\n\n⚠️  IMPORTANT: This value must match pubsub.clusterEventsTopic in the application chart\n\n🔗 More info: Topic will be created automatically by this chart" -}}
{{- end -}}

{{- /* Validate service account configuration consistency */ -}}
{{- if not .Values.serviceAccount.name -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: serviceAccount.name\n\n📋 REQUIRED ACTION:\n   Set serviceAccount.name for the GCP service account\n\n💡 EXAMPLE:\n   serviceAccount:\n     name: \"cls-backend\"\n\n⚠️  IMPORTANT: This value must match serviceAccount.gcpServiceAccountName in the application chart\n\n🔗 More info: Service account will be created automatically with necessary IAM roles" -}}
{{- end -}}

{{- /* Validate Workload Identity configuration consistency */ -}}
{{- if .Values.workloadIdentity.enabled -}}
{{- if not .Values.workloadIdentity.kubernetesNamespace -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: workloadIdentity.kubernetesNamespace\n\n📋 REQUIRED ACTION:\n   Set workloadIdentity.kubernetesNamespace when Workload Identity is enabled\n\n💡 EXAMPLE:\n   workloadIdentity:\n     enabled: true\n     kubernetesNamespace: \"cls-system\"\n\n⚠️  IMPORTANT: This value must match namespace.name in the application chart\n\n🔗 More info: This is the Kubernetes namespace where the application will be deployed" -}}
{{- end -}}
{{- if not .Values.workloadIdentity.kubernetesServiceAccount -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: workloadIdentity.kubernetesServiceAccount\n\n📋 REQUIRED ACTION:\n   Set workloadIdentity.kubernetesServiceAccount when Workload Identity is enabled\n\n💡 EXAMPLE:\n   workloadIdentity:\n     enabled: true\n     kubernetesServiceAccount: \"cls-backend\"\n\n⚠️  IMPORTANT: This value must match serviceAccount.name in the application chart\n\n🔗 More info: This is the Kubernetes service account that will impersonate the GCP service account" -}}
{{- end -}}
{{- end -}}

{{- /* Validation messages for cross-chart consistency */ -}}
{{- $dbInstance := .Values.database.instance.name -}}
{{- $dbName := .Values.database.database.name -}}
{{- $dbUser := .Values.database.user.name -}}
{{- $secretName := .Values.database.user.passwordSecret.name -}}
{{- $topic := .Values.pubsub.clusterEventsTopic -}}
{{- $saName := .Values.serviceAccount.name -}}
{{- $namespace := .Values.workloadIdentity.kubernetesNamespace -}}
{{- $ksa := .Values.workloadIdentity.kubernetesServiceAccount -}}

{{- if ne $dbInstance "cls-backend-db" -}}
{{- printf "\nWARNING: database.instance.name='%s' should typically be 'cls-backend-db' to match application chart defaults" $dbInstance | fail -}}
{{- end -}}

{{- if ne $dbName "cls_backend" -}}
{{- printf "\nWARNING: database.database.name='%s' should typically be 'cls_backend' to match application chart defaults" $dbName | fail -}}
{{- end -}}

{{- if ne $dbUser "cls_user" -}}
{{- printf "\nWARNING: database.user.name='%s' should typically be 'cls_user' to match application chart defaults" $dbUser | fail -}}
{{- end -}}

{{- if ne $secretName "cls-backend-db-password" -}}
{{- printf "\nWARNING: database.user.passwordSecret.name='%s' should typically be 'cls-backend-db-password' to match application chart defaults" $secretName | fail -}}
{{- end -}}

{{- if ne $topic "cluster-events" -}}
{{- printf "\nWARNING: pubsub.clusterEventsTopic='%s' should typically be 'cluster-events' to match application chart defaults" $topic | fail -}}
{{- end -}}

{{- if ne $saName "cls-backend" -}}
{{- printf "\nWARNING: serviceAccount.name='%s' should typically be 'cls-backend' to match application chart defaults" $saName | fail -}}
{{- end -}}

{{- if .Values.workloadIdentity.enabled -}}
{{- if ne $namespace "cls-system" -}}
{{- printf "\nWARNING: workloadIdentity.kubernetesNamespace='%s' should typically be 'cls-system' to match application chart defaults" $namespace | fail -}}
{{- end -}}
{{- if ne $ksa "cls-backend" -}}
{{- printf "\nWARNING: workloadIdentity.kubernetesServiceAccount='%s' should typically be 'cls-backend' to match application chart defaults" $ksa | fail -}}
{{- end -}}
{{- end -}}
{{- end }}