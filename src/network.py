# ************************************************************************** #
#   Copyright © hi@allali.me                                                 #
#                                                                            #
#   File    : network.py                                                     #
#   Project : p2p                                                            #
#   License : MIT                                                            #
#                                                                            #
#   Created: 2025/01/24 01:30:14 by aallali                                  #
#   Updated: 2025/01/24 01:34:08 by aallali                                  #
# ************************************************************************** #

import socket
import sys

def close_socket(sock):
    try:
        sock.settimeout(2)
        sock.shutdown(socket.SHUT_RDWR)
        sock.close()
    except socket.error:
        pass
    finally:
        sys.exit(0)
