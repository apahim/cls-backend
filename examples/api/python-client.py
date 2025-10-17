#!/usr/bin/env python3
"""
CLS Backend Python Client Example

This example demonstrates how to interact with the CLS Backend API using Python.
It includes error handling, authentication, and all major API operations.

Requirements:
    pip install requests

Usage:
    python python-client.py
"""

import json
import time
import uuid
from typing import Dict, List, Optional, Any
from datetime import datetime, timezone

import requests


class CLSClient:
    """Python client for CLS Backend API with simplified single-tenant architecture."""

    def __init__(self, base_url: str, user_email: str, timeout: int = 30):
        """
        Initialize the CLS client.

        Args:
            base_url: Base URL of the CLS Backend (e.g., http://localhost:8080)
            user_email: User email for authentication header
            timeout: Request timeout in seconds
        """
        self.base_url = base_url.rstrip('/')
        self.api_url = f"{self.base_url}/api/v1"
        self.user_email = user_email
        self.timeout = timeout
        self.session = requests.Session()

        # Set default headers
        self.session.headers.update({
            'Content-Type': 'application/json',
            'X-User-Email': self.user_email,
            'User-Agent': 'cls-python-client/1.0.0'
        })

    def _request(self, method: str, path: str, data: Optional[Dict] = None,
                params: Optional[Dict] = None) -> Dict:
        """
        Make an HTTP request to the API.

        Args:
            method: HTTP method (GET, POST, PUT, DELETE)
            path: API path (without /api/v1 prefix)
            data: JSON data for request body
            params: Query parameters

        Returns:
            Response JSON data

        Raises:
            CLSAPIError: If the API returns an error
            requests.RequestException: For network errors
        """
        url = f"{self.api_url}{path}"

        try:
            response = self.session.request(
                method=method,
                url=url,
                json=data,
                params=params,
                timeout=self.timeout
            )

            # Handle different status codes
            if response.status_code == 200:
                return response.json()
            elif response.status_code == 201:
                return response.json()
            elif response.status_code == 404:
                raise CLSAPIError(f"Resource not found: {path}", 404, response.json())
            elif response.status_code == 409:
                raise CLSAPIError(f"Conflict: {response.json().get('error', 'Unknown conflict')}", 409, response.json())
            elif response.status_code >= 400:
                error_data = response.json() if response.content else {}
                raise CLSAPIError(f"API Error {response.status_code}: {error_data.get('error', 'Unknown error')}",
                                response.status_code, error_data)
            else:
                response.raise_for_status()
                return response.json()

        except requests.RequestException as e:
            raise CLSAPIError(f"Network error: {str(e)}", 0, {})

    # Health and Info endpoints
    def health_check(self) -> Dict:
        """Get service health status."""
        return self._request('GET', '/health')

    def get_info(self) -> Dict:
        """Get service information."""
        return self._request('GET', '/info')

    # Cluster management
    def list_clusters(self, limit: int = 50, offset: int = 0,
                     platform: Optional[str] = None, status: Optional[str] = None) -> Dict:
        """
        List clusters with optional filtering and pagination.

        Args:
            limit: Maximum number of results (1-100)
            offset: Number of results to skip
            platform: Filter by platform (gcp, aws, azure)
            status: Filter by status phase

        Returns:
            Dictionary with clusters, pagination info
        """
        params = {'limit': limit, 'offset': offset}
        if platform:
            params['platform'] = platform
        if status:
            params['status'] = status

        return self._request('GET', '/clusters', params=params)

    def create_cluster(self, name: str, platform_type: str,
                      platform_config: Optional[Dict] = None,
                      target_project_id: Optional[str] = None,
                      spec: Optional[Dict] = None) -> Dict:
        """
        Create a new cluster.

        Args:
            name: Cluster name
            platform_type: Platform type (gcp, aws, azure)
            platform_config: Platform-specific configuration
            target_project_id: Target project ID
            spec: Complete cluster specification (overrides other params)

        Returns:
            Created cluster data
        """
        if spec is None:
            spec = {
                'platform': {
                    'type': platform_type
                }
            }

            if platform_config:
                spec['platform'][platform_type] = platform_config

        data = {
            'name': name,
            'spec': spec
        }

        if target_project_id:
            data['target_project_id'] = target_project_id

        return self._request('POST', '/clusters', data=data)

    def get_cluster(self, cluster_id: str) -> Dict:
        """
        Get cluster details by ID.

        Args:
            cluster_id: Cluster UUID

        Returns:
            Cluster data with aggregated status
        """
        return self._request('GET', f'/clusters/{cluster_id}')

    def update_cluster(self, cluster_id: str, spec: Dict) -> Dict:
        """
        Update cluster specification.

        Args:
            cluster_id: Cluster UUID
            spec: New cluster specification

        Returns:
            Updated cluster data
        """
        data = {'spec': spec}
        return self._request('PUT', f'/clusters/{cluster_id}', data=data)

    def delete_cluster(self, cluster_id: str, force: bool = False) -> Dict:
        """
        Delete a cluster.

        Args:
            cluster_id: Cluster UUID
            force: Force delete regardless of state

        Returns:
            Deletion confirmation
        """
        params = {'force': 'true'} if force else {}
        return self._request('DELETE', f'/clusters/{cluster_id}', params=params)

    def get_cluster_status(self, cluster_id: str) -> Dict:
        """
        Get detailed cluster status including controller status.

        Args:
            cluster_id: Cluster UUID

        Returns:
            Detailed status information
        """
        return self._request('GET', f'/clusters/{cluster_id}/status')

    def update_controller_status(self, cluster_id: str, controller_name: str,
                               observed_generation: int, conditions: List[Dict],
                               metadata: Optional[Dict] = None,
                               last_error: Optional[Dict] = None) -> Dict:
        """
        Update controller status (for controller implementations).

        Args:
            cluster_id: Cluster UUID
            controller_name: Name of the controller
            observed_generation: Generation observed by controller
            conditions: List of status conditions
            metadata: Optional metadata
            last_error: Optional error information

        Returns:
            Update confirmation
        """
        data = {
            'controller_name': controller_name,
            'observed_generation': observed_generation,
            'conditions': conditions
        }

        if metadata:
            data['metadata'] = metadata
        if last_error:
            data['last_error'] = last_error

        return self._request('PUT', f'/clusters/{cluster_id}/status', data=data)

    # Convenience methods
    def wait_for_cluster_phase(self, cluster_id: str, target_phase: str,
                              timeout: int = 300, poll_interval: int = 10) -> Dict:
        """
        Wait for cluster to reach a specific phase.

        Args:
            cluster_id: Cluster UUID
            target_phase: Target phase (Pending, Progressing, Ready, Failed)
            timeout: Maximum time to wait in seconds
            poll_interval: Polling interval in seconds

        Returns:
            Final cluster data

        Raises:
            TimeoutError: If timeout is reached
            CLSAPIError: If cluster reaches Failed state (unless that's the target)
        """
        start_time = time.time()

        while time.time() - start_time < timeout:
            cluster = self.get_cluster(cluster_id)
            current_phase = cluster['status']['phase']

            print(f"Cluster {cluster_id} phase: {current_phase}")

            if current_phase == target_phase:
                return cluster

            if current_phase == 'Failed' and target_phase != 'Failed':
                raise CLSAPIError(f"Cluster reached Failed state: {cluster['status']['message']}", 0, cluster)

            time.sleep(poll_interval)

        raise TimeoutError(f"Timeout waiting for cluster {cluster_id} to reach phase {target_phase}")

    def create_simple_gcp_cluster(self, name: str, project_id: str,
                                 region: str = 'us-central1') -> Dict:
        """
        Create a simple GCP cluster with minimal configuration.

        Args:
            name: Cluster name
            project_id: GCP project ID
            region: GCP region

        Returns:
            Created cluster data
        """
        return self.create_cluster(
            name=name,
            platform_type='gcp',
            platform_config={
                'projectID': project_id,
                'region': region
            }
        )


class CLSAPIError(Exception):
    """Exception raised for CLS API errors."""

    def __init__(self, message: str, status_code: int, response_data: Dict):
        super().__init__(message)
        self.message = message
        self.status_code = status_code
        self.response_data = response_data


def main():
    """Example usage of the CLS Python client."""

    # Initialize client
    client = CLSClient(
        base_url='http://localhost:8080',
        user_email='user@example.com'
    )

    print("🚀 CLS Backend Python Client Example")
    print("=" * 50)

    try:
        # 1. Health check
        print("1. Checking service health...")
        health = client.health_check()
        print(f"   Status: {health['status']}")
        print(f"   Components: {health['components']}")
        print()

        # 2. Service info
        print("2. Getting service information...")
        info = client.get_info()
        print(f"   Service: {info['service']} v{info['version']}")
        print(f"   Environment: {info['environment']}")
        print()

        # 3. List existing clusters
        print("3. Listing existing clusters...")
        clusters_response = client.list_clusters()
        print(f"   Found {clusters_response['total']} clusters")
        print()

        # 4. Create a simple cluster
        print("4. Creating a simple GCP cluster...")
        cluster_name = f"python-example-{uuid.uuid4().hex[:8]}"
        cluster = client.create_simple_gcp_cluster(
            name=cluster_name,
            project_id='my-test-project',
            region='us-central1'
        )
        cluster_id = cluster['id']
        print(f"   Created cluster: {cluster_id}")
        print(f"   Name: {cluster['name']}")
        print(f"   Phase: {cluster['status']['phase']}")
        print()

        # 5. Get cluster details
        print("5. Getting cluster details...")
        cluster_details = client.get_cluster(cluster_id)
        print(f"   Generation: {cluster_details['generation']}")
        print(f"   Created by: {cluster_details['created_by']}")
        print(f"   Status conditions: {len(cluster_details['status']['conditions'])}")
        print()

        # 6. Update cluster
        print("6. Updating cluster...")
        updated_spec = {
            'platform': {
                'type': 'gcp',
                'gcp': {
                    'projectID': 'my-test-project',
                    'region': 'us-west1'  # Changed region
                }
            }
        }
        updated_cluster = client.update_cluster(cluster_id, updated_spec)
        print(f"   Updated generation: {updated_cluster['generation']}")
        print(f"   New region: {updated_cluster['spec']['platform']['gcp']['region']}")
        print()

        # 7. Simulate controller status update
        print("7. Simulating controller status update...")
        client.update_controller_status(
            cluster_id=cluster_id,
            controller_name='python-example-controller',
            observed_generation=updated_cluster['generation'],
            conditions=[
                {
                    'type': 'Available',
                    'status': 'True',
                    'lastTransitionTime': datetime.now(timezone.utc).isoformat(),
                    'reason': 'WorkCompleted',
                    'message': 'Python example controller completed successfully'
                }
            ],
            metadata={
                'platform': 'gcp',
                'region': 'us-west1',
                'controller_version': '1.0.0'
            }
        )
        print("   Controller status updated successfully")
        print()

        # 8. Get detailed status
        print("8. Getting detailed cluster status...")
        status = client.get_cluster_status(cluster_id)
        print(f"   Overall phase: {status['status']['phase']}")
        print(f"   Controllers reporting: {len(status['controllers'])}")
        for controller in status['controllers']:
            print(f"     - {controller['controller_name']}: {controller['conditions'][0]['status']}")
        print()

        # 9. List clusters with filtering
        print("9. Listing GCP clusters...")
        gcp_clusters = client.list_clusters(platform='gcp', limit=10)
        print(f"   Found {gcp_clusters['total']} GCP clusters")
        print()

        # 10. Delete cluster
        print("10. Deleting cluster...")
        client.delete_cluster(cluster_id, force=True)
        print(f"    Cluster {cluster_id} deleted successfully")
        print()

        print("✅ All examples completed successfully!")

    except CLSAPIError as e:
        print(f"❌ API Error: {e.message}")
        print(f"   Status Code: {e.status_code}")
        if e.response_data:
            print(f"   Details: {json.dumps(e.response_data, indent=2)}")
    except Exception as e:
        print(f"❌ Unexpected error: {str(e)}")


if __name__ == '__main__':
    main()