#!/bin/bash

# CLS Backend API Examples using cURL
# This script demonstrates all major API operations with the simplified single-tenant architecture

set -e

# Configuration
API_BASE=${API_BASE:-"http://localhost:8080/api/v1"}
USER_EMAIL=${USER_EMAIL:-"user@example.com"}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if API is accessible
check_api() {
    log "Checking API accessibility..."
    if curl -s "${API_BASE%/api/v1}/health" > /dev/null; then
        success "API is accessible"
    else
        error "API is not accessible at ${API_BASE%/api/v1}"
        error "Make sure CLS Backend is running and accessible"
        exit 1
    fi
}

# 1. Health Check
health_check() {
    log "1. Health Check"
    echo "GET ${API_BASE%/api/v1}/health"

    response=$(curl -s "${API_BASE%/api/v1}/health")
    echo "$response" | jq '.'

    status=$(echo "$response" | jq -r '.status')
    if [ "$status" = "healthy" ]; then
        success "Service is healthy"
    else
        warn "Service health: $status"
    fi
    echo
}

# 2. Service Information
service_info() {
    log "2. Service Information"
    echo "GET ${API_BASE%/api/v1}/info"

    curl -s "${API_BASE%/api/v1}/info" | jq '.'
    echo
}

# 3. List Clusters (initially empty)
list_clusters() {
    log "3. List Clusters"
    echo "GET $API_BASE/clusters"
    echo "Headers: X-User-Email: $USER_EMAIL"

    response=$(curl -s -H "X-User-Email: $USER_EMAIL" "$API_BASE/clusters")
    echo "$response" | jq '.'

    count=$(echo "$response" | jq '.total')
    log "Found $count clusters"
    echo
}

# 4. Create a simple cluster
create_simple_cluster() {
    log "4. Create Simple Cluster"
    echo "POST $API_BASE/clusters"

    payload='{
        "name": "simple-test-cluster",
        "spec": {
            "platform": {
                "type": "gcp"
            }
        }
    }'

    echo "Payload:"
    echo "$payload" | jq '.'

    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -H "X-User-Email: $USER_EMAIL" \
        -d "$payload" \
        "$API_BASE/clusters")

    echo "Response:"
    echo "$response" | jq '.'

    SIMPLE_CLUSTER_ID=$(echo "$response" | jq -r '.id')
    if [ "$SIMPLE_CLUSTER_ID" != "null" ]; then
        success "Created cluster: $SIMPLE_CLUSTER_ID"
    else
        error "Failed to create cluster"
    fi
    echo
}

# 5. Create a detailed GCP cluster
create_detailed_cluster() {
    log "5. Create Detailed GCP Cluster"
    echo "POST $API_BASE/clusters"

    payload='{
        "name": "production-gcp-cluster",
        "target_project_id": "my-gcp-project",
        "spec": {
            "infraID": "prod-cluster-infra",
            "platform": {
                "type": "gcp",
                "gcp": {
                    "projectID": "my-gcp-project",
                    "region": "us-central1",
                    "zone": "us-central1-a"
                }
            },
            "release": {
                "image": "quay.io/openshift-release-dev/ocp-release:4.14.0",
                "version": "4.14.0"
            },
            "networking": {
                "clusterNetwork": [
                    {"cidr": "10.128.0.0/14", "hostPrefix": 23}
                ],
                "serviceNetwork": ["172.30.0.0/16"]
            },
            "dns": {
                "baseDomain": "example.com"
            }
        }
    }'

    echo "Payload:"
    echo "$payload" | jq '.'

    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -H "X-User-Email: $USER_EMAIL" \
        -d "$payload" \
        "$API_BASE/clusters")

    echo "Response:"
    echo "$response" | jq '.'

    DETAILED_CLUSTER_ID=$(echo "$response" | jq -r '.id')
    if [ "$DETAILED_CLUSTER_ID" != "null" ]; then
        success "Created detailed cluster: $DETAILED_CLUSTER_ID"
    else
        error "Failed to create detailed cluster"
    fi
    echo
}

# 6. Get cluster details
get_cluster() {
    if [ -z "$SIMPLE_CLUSTER_ID" ]; then
        warn "No cluster ID available, skipping get cluster example"
        return
    fi

    log "6. Get Cluster Details"
    echo "GET $API_BASE/clusters/$SIMPLE_CLUSTER_ID"

    response=$(curl -s -H "X-User-Email: $USER_EMAIL" \
        "$API_BASE/clusters/$SIMPLE_CLUSTER_ID")

    echo "$response" | jq '.'

    phase=$(echo "$response" | jq -r '.status.phase')
    log "Cluster phase: $phase"
    echo
}

# 7. Update cluster
update_cluster() {
    if [ -z "$SIMPLE_CLUSTER_ID" ]; then
        warn "No cluster ID available, skipping update cluster example"
        return
    fi

    log "7. Update Cluster"
    echo "PUT $API_BASE/clusters/$SIMPLE_CLUSTER_ID"

    payload='{
        "spec": {
            "platform": {
                "type": "gcp",
                "gcp": {
                    "projectID": "updated-project",
                    "region": "us-west1"
                }
            }
        }
    }'

    echo "Update payload:"
    echo "$payload" | jq '.'

    response=$(curl -s -X PUT \
        -H "Content-Type: application/json" \
        -H "X-User-Email: $USER_EMAIL" \
        -d "$payload" \
        "$API_BASE/clusters/$SIMPLE_CLUSTER_ID")

    echo "Response:"
    echo "$response" | jq '.'

    generation=$(echo "$response" | jq -r '.generation')
    log "Updated cluster generation: $generation"
    echo
}

# 8. Get cluster status
get_cluster_status() {
    if [ -z "$SIMPLE_CLUSTER_ID" ]; then
        warn "No cluster ID available, skipping get status example"
        return
    fi

    log "8. Get Cluster Status"
    echo "GET $API_BASE/clusters/$SIMPLE_CLUSTER_ID/status"

    response=$(curl -s -H "X-User-Email: $USER_EMAIL" \
        "$API_BASE/clusters/$SIMPLE_CLUSTER_ID/status")

    echo "$response" | jq '.'
    echo
}

# 9. Simulate controller status update
update_controller_status() {
    if [ -z "$SIMPLE_CLUSTER_ID" ]; then
        warn "No cluster ID available, skipping controller status example"
        return
    fi

    log "9. Update Controller Status (Simulated)"
    echo "PUT $API_BASE/clusters/$SIMPLE_CLUSTER_ID/status"

    payload='{
        "controller_name": "example-controller",
        "observed_generation": 2,
        "conditions": [
            {
                "type": "Available",
                "status": "True",
                "lastTransitionTime": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'",
                "reason": "WorkCompleted",
                "message": "Example controller completed successfully"
            }
        ],
        "metadata": {
            "platform": "gcp",
            "region": "us-west1"
        }
    }'

    echo "Controller status payload:"
    echo "$payload" | jq '.'

    response=$(curl -s -X PUT \
        -H "Content-Type: application/json" \
        -H "X-User-Email: controller@system.local" \
        -d "$payload" \
        "$API_BASE/clusters/$SIMPLE_CLUSTER_ID/status")

    echo "Response:"
    echo "$response" | jq '.'
    echo
}

# 10. List clusters with pagination
list_clusters_paginated() {
    log "10. List Clusters with Pagination"
    echo "GET $API_BASE/clusters?limit=1&offset=0"

    response=$(curl -s -H "X-User-Email: $USER_EMAIL" \
        "$API_BASE/clusters?limit=1&offset=0")

    echo "$response" | jq '.'

    total=$(echo "$response" | jq '.total')
    limit=$(echo "$response" | jq '.limit')
    offset=$(echo "$response" | jq '.offset')
    log "Showing $limit results from offset $offset (total: $total)"
    echo
}

# 11. List clusters with filtering
list_clusters_filtered() {
    log "11. List Clusters with Filtering"
    echo "GET $API_BASE/clusters?platform=gcp&status=Pending"

    response=$(curl -s -H "X-User-Email: $USER_EMAIL" \
        "$API_BASE/clusters?platform=gcp&status=Pending")

    echo "$response" | jq '.'
    echo
}

# 12. Delete cluster
delete_cluster() {
    if [ -z "$DETAILED_CLUSTER_ID" ]; then
        warn "No detailed cluster ID available, skipping delete example"
        return
    fi

    log "12. Delete Cluster"
    echo "DELETE $API_BASE/clusters/$DETAILED_CLUSTER_ID"

    response=$(curl -s -X DELETE \
        -H "X-User-Email: $USER_EMAIL" \
        "$API_BASE/clusters/$DETAILED_CLUSTER_ID")

    echo "Response:"
    echo "$response" | jq '.'
    success "Deleted cluster: $DETAILED_CLUSTER_ID"
    echo
}

# 13. Force delete cluster
force_delete_cluster() {
    if [ -z "$SIMPLE_CLUSTER_ID" ]; then
        warn "No simple cluster ID available, skipping force delete example"
        return
    fi

    log "13. Force Delete Cluster"
    echo "DELETE $API_BASE/clusters/$SIMPLE_CLUSTER_ID?force=true"

    response=$(curl -s -X DELETE \
        -H "X-User-Email: $USER_EMAIL" \
        "$API_BASE/clusters/$SIMPLE_CLUSTER_ID?force=true")

    echo "Response:"
    echo "$response" | jq '.'
    success "Force deleted cluster: $SIMPLE_CLUSTER_ID"
    echo
}

# 14. Error handling examples
error_examples() {
    log "14. Error Handling Examples"

    # Invalid cluster ID (404)
    log "Example: Get non-existent cluster (404)"
    echo "GET $API_BASE/clusters/non-existent-id"

    response=$(curl -s -w "%{http_code}" -H "X-User-Email: $USER_EMAIL" \
        "$API_BASE/clusters/non-existent-id")

    http_code="${response: -3}"
    body="${response%???}"

    echo "HTTP Status: $http_code"
    echo "Response:"
    echo "$body" | jq '.'
    echo

    # Invalid JSON (400)
    log "Example: Invalid JSON payload (400)"
    echo "POST $API_BASE/clusters"

    response=$(curl -s -w "%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -H "X-User-Email: $USER_EMAIL" \
        -d '{"invalid": json}' \
        "$API_BASE/clusters")

    http_code="${response: -3}"
    body="${response%???}"

    echo "HTTP Status: $http_code"
    echo "Response:"
    echo "$body" | jq '.'
    echo
}

# Main execution
main() {
    log "Starting CLS Backend API Examples"
    log "API Base URL: $API_BASE"
    log "User Email: $USER_EMAIL"
    echo

    # Check if jq is available
    if ! command -v jq &> /dev/null; then
        error "jq is required for JSON formatting. Please install jq."
        exit 1
    fi

    # Run examples
    check_api
    health_check
    service_info
    list_clusters
    create_simple_cluster
    create_detailed_cluster
    get_cluster
    update_cluster
    get_cluster_status
    update_controller_status
    list_clusters_paginated
    list_clusters_filtered
    delete_cluster
    force_delete_cluster
    error_examples

    success "All API examples completed successfully!"
    log "Note: Some clusters may still exist. Check with 'list_clusters' if needed."
}

# Run if executed directly
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    main "$@"
fi