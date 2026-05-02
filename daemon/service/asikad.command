#!/bin/bash
# Asika Dashboard Launcher for macOS
# Double-click this .command file in Finder to launch the Asika dashboard.
# It opens Terminal, starts asikad, and opens the WebUI in your browser.

cd "$(dirname "$0")"

echo "Starting Asika Dashboard..."
echo ""
/usr/local/bin/asikad --desktop

# Keep the terminal open if asikad exits unexpectedly
echo ""
echo "Asika has stopped. Press any key to close this window."
read -n 1