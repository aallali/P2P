# ************************************************************************** #
#   Copyright © hi@allali.me                                                 #
#                                                                            #
#   File    : commands.py                                                    #
#   Project : p2p                                                            #
#   License : MIT                                                            #
#                                                                            #
#   Created: 2025/01/24 00:42:21 by aallali                                  #
#   Updated: 2025/01/24 01:33:32 by aallali                                  #
# ************************************************************************** #

import os
from .network import close_socket
from .logger import log
from .files import send_file
import time

class CommandHandler:
    def __init__(self, sock, received_dir):
        self.sock = sock
        self.current_file = None
        self.received_dir = received_dir

    def execute(self, message):
        if message.startswith("/file "):
            self.select_file(message.split(" ", 1)[1].strip())
        elif message == "/send":
            self.resend_file()
        elif message == "/clear":
            self.clear_screen()
        elif message == "/info":
            self.show_file_info()
        elif message in ["/close", "/c"]:
            self.close_socket()
        else:
            self.send_chat(message)

    def select_file(self, file_path):
        if os.path.exists(file_path):
            self.current_file = file_path
            log(f"Selected: {file_path}", "ME", "FILE")
            self.send_file()
        else:
            log(f"File not found: {file_path}", "SYSTEM", "ERROR")

    def resend_file(self):
        if self.current_file and os.path.exists(self.current_file):
            log(f"Resending: {self.current_file}", "ME", "FILE")
            self.send_file()
        else:
            log("No file selected or file not found", "SYSTEM", "ERROR")

    def send_file(self):
        # Call the existing `send_file` function
        send_file(self.sock, self.current_file)

    def clear_screen(self):
        os.system("clear" if os.name == "posix" else "cls")
        log("Screen cleared", "SYSTEM", "INFO")

    def show_file_info(self):
        if self.current_file:
            file_info = os.stat(self.current_file)
            log(f"File: {self.current_file}", "SYSTEM", "INFO")
            log(f"Size: {file_info.st_size} bytes", "SYSTEM", "INFO")
            log(f"Modified: {time.ctime(file_info.st_mtime)}", "SYSTEM", "INFO")
            # add full path 
            log(f"Location: {os.path.abspath(self.current_file)}", "SYSTEM", "INFO")
        else:
            log("No file selected", "SYSTEM", "ERROR")

    def close_socket(self):
        log("Closing connection", "SYSTEM", "INFO")
        close_socket(self.sock)

    def send_chat(self, message):
        log(message, "ME", "CHAT")
        self.sock.sendall(message.encode())
