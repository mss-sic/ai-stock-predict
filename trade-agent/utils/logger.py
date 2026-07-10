"""Logging setup for the agent."""

import logging
import os
import sys
from logging.handlers import RotatingFileHandler


def setup_logging(level="INFO", log_file="agent.log"):
    """Configure rotating file + console logging."""
    log_level = getattr(logging, level.upper(), logging.INFO)

    root = logging.getLogger("agent")
    root.setLevel(log_level)

    # Console handler
    console = logging.StreamHandler(sys.stdout)
    console.setLevel(log_level)
    console.setFormatter(logging.Formatter(
        "%(asctime)s [%(name)s] %(levelname)s: %(message)s",
        datefmt="%H:%M:%S",
    ))
    root.addHandler(console)

    # File handler (rotating, 10MB max, 5 backups)
    log_path = os.path.join(os.getcwd(), log_file)
    file_handler = RotatingFileHandler(
        log_path, maxBytes=10 * 1024 * 1024, backupCount=5, encoding="utf-8"
    )
    file_handler.setLevel(log_level)
    file_handler.setFormatter(logging.Formatter(
        "%(asctime)s [%(name)s] %(levelname)s: %(message)s",
    ))
    root.addHandler(file_handler)

    return root
