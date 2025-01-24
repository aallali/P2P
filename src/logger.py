# ************************************************************************** #
#   Copyright © hi@allali.me                                                 #
#                                                                            #
#   File    : logger.py                                                      #
#   Project : p2p                                                            #
#   License : MIT                                                            #
#                                                                            #
#   Created: 2025/01/23 19:56:40 by aallali                                  #
#   Updated: 2025/01/24 01:34:04 by aallali                                  #
# ************************************************************************** #

import logging
from datetime import datetime

class Colors:
    ME = "\033[96m"  # Cyan for my messages
    HIM = "\033[95m"  # Purple for their messages
    INFO = "\033[94m"  # Blue for system info
    SUCCESS = "\033[92m"  # Green for success
    WARNING = "\033[93m"  # Yellow for warnings
    ERROR = "\033[91m"  # Red for errors
    RESET = "\033[0m"

class P2PFormatter(logging.Formatter):
    def format(self, record):
        timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

        # Determine role and assign color
        if hasattr(record, "role"):
            if record.role == "ME":
                color = Colors.ME
            elif record.role == "HIM":
                color = Colors.HIM
            elif record.role == "SYSTEM":
                color = Colors.WARNING
            else:
                color = Colors.RESET
        else:
            color = Colors.RESET

        # Determine message type color
        if record.levelname == "ERROR":
            msg_color = Colors.ERROR
        elif record.levelname == "WARNING":
            msg_color = Colors.WARNING
        elif record.levelname == "SUCCESS":
            msg_color = Colors.SUCCESS
        else:
            msg_color = color  # Default to role color

        # Format the message with colors
        formatted_message = f"{color}[{timestamp}][{record.role}][{record.levelname}] {msg_color}{record.getMessage()}{Colors.RESET}"
        return formatted_message

def setup_logger(persistent_logging):
    logger = logging.getLogger("p2p")
    logger.setLevel(logging.DEBUG)

    # Console handler with colored output
    ch = logging.StreamHandler()
    ch.setLevel(logging.DEBUG)
    ch.setFormatter(P2PFormatter())
    logger.addHandler(ch)

    if persistent_logging:
        # File handler without colors
        fh = logging.FileHandler("p2p.log")
        fh.setLevel(logging.DEBUG)
        fh.setFormatter(
            logging.Formatter("[%(asctime)s][%(role)s][%(levelname)s] %(message)s")
        )
        logger.addHandler(fh)

    return logger

def log(msg, role="SYSTEM", msg_type="INFO"):
    logger = logging.getLogger("p2p")
    level = logging.ERROR if msg_type == "ERROR" else logging.INFO
    logger.log(level, msg, extra={"role": role, "msg_type": msg_type})
