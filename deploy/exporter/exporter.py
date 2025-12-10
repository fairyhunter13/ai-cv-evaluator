import time
import docker
from prometheus_client import start_http_server, Gauge

# Create a Prometheus Gauge for container metadata
# Labels: id (short 12-char), name (container name), image, service (compose service), state
CONTAINER_META = Gauge('container_meta_info', 'Container metadata info', 
                       ['id', 'name', 'image', 'com_docker_compose_service', 'state', 'full_id'])

def collect_metrics():
    try:
        client = docker.from_env()
        containers = client.containers.list(all=True)
        
        # Clear previous metrics to handle stopped/removed containers
        CONTAINER_META.clear()
        
        for container in containers:
            try:
                # Extract relevant info
                # cAdvisor id format often matches /system.slice/docker-<long_id>.scope or just the ID
                # We expose both short and full ID to allow flexible joining
                full_id = container.id
                short_id = container.id[:12]
                name = container.name.lstrip('/')
                image = container.image.tags[0] if container.image.tags else container.image.id
                state = container.status
                
                # Extract Docker Compose Service label if present
                labels = container.labels
                service = labels.get('com.docker.compose.service', name) # Fallback to name if not a compose service
                
                # Set the metric
                CONTAINER_META.labels(
                    id=short_id,
                    name=name,
                    image=image,
                    com_docker_compose_service=service,
                    state=state,
                    full_id=full_id
                ).set(1)
                
            except Exception as e:
                print(f"Error processing container {container.short_id}: {e}")
                
    except Exception as e:
        print(f"Error connecting to Docker: {e}")

if __name__ == '__main__':
    # Start up the server to expose the metrics.
    print("Starting Docker Meta Exporter on port 8000...")
    start_http_server(8000)
    
    # Refresh metrics every 15 seconds
    while True:
        collect_metrics()
        time.sleep(15)
