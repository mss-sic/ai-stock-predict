"""Agent authentication — loads config and provides token for API calls."""

import os
import yaml


def load_config(config_path=None):
    """Load agent configuration from YAML file.
    
    Looks for config in this order:
    1. config_path argument
    2. AGENT_CONFIG env var
    3. ./config.yaml (next to agent.py)
    4. ~/.agent_config.yaml
    """
    paths = []
    if config_path:
        paths.append(config_path)
    if os.environ.get("AGENT_CONFIG"):
        paths.append(os.environ["AGENT_CONFIG"])
    paths.append(os.path.join(os.getcwd(), "config.yaml"))
    paths.append(os.path.expanduser("~/.agent_config.yaml"))

    for p in paths:
        if os.path.exists(p):
            with open(p, 'r') as f:
                return yaml.safe_load(f)

    raise FileNotFoundError(
        "No config.yaml found. Copy config.example.yaml to config.yaml and fill in your values."
    )


def get_token(config=None):
    """Extract agent token from config."""
    if config is None:
        config = load_config()
    token = config.get("agent_token", "")
    if not token:
        raise ValueError("agent_token not set in config.yaml")
    return token


def get_server_url(config=None):
    """Extract server URL from config."""
    if config is None:
        config = load_config()
    return config.get("server_url", "http://localhost:8080").rstrip("/")
