from google.cloud import resourcemanager_v3
import os

os.environ['GOOGLE_APPLICATION_CREDENTIALS'] = os.path.expanduser('~/.config/oidc2gcloud/riken-authentik-config.json')

client = resourcemanager_v3.ProjectsClient()
project = client.get_project(name='projects/cloudlink-g01')
print(f"Project: {project.display_name}")
