import logging
import os
import sys

LOGLEVEL = os.environ.get("LOGLEVEL", "DEBUG").upper()
logger = logging.getLogger("pipeline")
logger.setLevel(LOGLEVEL)
logger.propagate = False

# Output Debug and Info logs to stdout, and above to stderr
stdout_handler = logging.StreamHandler(sys.stdout)
stdout_handler.setLevel(logging.DEBUG)
stdout_handler.addFilter(lambda record: record.levelno <= logging.INFO)
stderr_handler = logging.StreamHandler(sys.stderr)
stderr_handler.setLevel(logging.WARNING)

formatter = logging.Formatter("%(asctime)s - %(levelname)s - %(message)s")
stdout_handler.setFormatter(formatter)
stderr_handler.setFormatter(formatter)
logger.addHandler(stdout_handler)
logger.addHandler(stderr_handler)
