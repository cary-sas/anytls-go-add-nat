#!/bin/bash

set -e

# Configuration
VPS_IP="${VPS_IP:-}"
VPS_PORT="${VPS_PORT:-8443}"
PASSWORD="${PASSWORD:-}"
GATEWAY_IP="${GATEWAY_IP:-192.168.1.1}"
ROUTER_IFACE="${ROUTER_IFACE:-eth0}"  # Interface connected to external network
LAN_IFACE="${LAN_IFACE:-br-lan}"       # Interface for LAN devices

echo "=== Setup anytls Router Gateway ==="
echo ""
echo "Configuration:"
echo "  VPS IP: $VPS_IP"
echo "  VPS Port: $VPS_PORT"
echo "  Gateway IP: $GATEWAY_IP"
echo "  Router Interface: $ROUTER_IFACE"
echo "  LAN Interface: $LAN_IFACE"
echo ""

if [ -z "$VPS_IP" ] || [ -z "$PASSWORD" ]; then
    echo "Error: VPS_IP and PASSWORD must be set"
    echo ""
    echo "Usage:"
    echo "  export VPS_IP='your_vps_ip'"
    echo "  export PASSWORD='your_password'"
    echo "  bash setup_router.sh"
    exit 1
fi

# 1. Enable IP forwarding
echo "1. Enabling IP forwarding..."
echo 1 > /proc/sys/net/ipv4/ip_forward
echo "net.ipv4.ip_forward = 1" | tee /etc/sysctl.d/99-anytls.conf > /dev/null
sysctl -p /etc/sysctl.d/99-anytls.conf

# 2. Configure iptables NAT rules
echo "2. Configuring iptables..."

# Clear old rules
iptables -t nat -F OUTPUT 2>/dev/null || true
iptables -t nat -F PREROUTING 2>/dev/null || true
iptables -F FORWARD 2>/dev/null || true
iptables -t nat -F POSTROUTING 2>/dev/null || true

# === Router's own traffic (OUTPUT chain) ===
# Skip SSH to prevent disconnection
iptables -t nat -A OUTPUT -p tcp --dport 22 -j ACCEPT

# Skip connection to VPS server (prevent loop)
iptables -t nat -A OUTPUT -p tcp -d $VPS_IP --dport $VPS_PORT -j ACCEPT

# Skip loopback and local addresses
iptables -t nat -A OUTPUT -p tcp -d 127.0.0.1 -j ACCEPT
iptables -t nat -A OUTPUT -p tcp -d 192.168.1.0/24 -j ACCEPT
iptables -t nat -A OUTPUT -p tcp -d 10.0.0.0/8 -j ACCEPT
iptables -t nat -A OUTPUT -p tcp -d 172.16.0.0/12 -j ACCEPT

# Redirect all other TCP traffic to NAT proxy port
iptables -t nat -A OUTPUT -p tcp -j REDIRECT --to-port 3333

# === LAN devices' traffic (PREROUTING + FORWARD chains) ===
# Redirect LAN TCP traffic (except SSH) to NAT proxy port
iptables -t nat -A PREROUTING -i $LAN_IFACE -p tcp ! --dport 22 -j REDIRECT --to-port 3333

# Allow LAN forwarding
iptables -A FORWARD -i $LAN_IFACE -o $ROUTER_IFACE -j ACCEPT
iptables -A FORWARD -i $ROUTER_IFACE -o $LAN_IFACE -m state --state RELATED,ESTABLISHED -j ACCEPT

# NAT masquerade for outbound traffic
iptables -t nat -A POSTROUTING -o $ROUTER_IFACE -j MASQUERADE

# 3. Save iptables rules
echo "3. Saving iptables rules..."
if command -v iptables-save &> /dev/null; then
    mkdir -p /etc/iptables
    iptables-save > /etc/iptables/rules.v4
    # For persistence on reboot, you may want to use iptables-persistent
    if ! command -v netfilter-persistent &> /dev/null; then
        echo ""
        echo "⚠️  Consider installing 'iptables-persistent' for persistence:"
        echo "   apt-get install iptables-persistent"
    fi
fi

echo "4. iptables rules configured successfully"
echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "1. Start the anytls-client on the router:"
echo "   ./anytls-client \\"
echo "     -s $VPS_IP:$VPS_PORT \\"
echo "     -p '$PASSWORD' \\"
echo "     -socks 127.0.0.1:1080 \\"
echo "     -nat 0.0.0.0:3333"
echo ""
echo "2. Test from a LAN device:"
echo "   curl https://ifconfig.me"
echo ""
echo "3. Check anytls-client logs:"
echo "   tail -f /var/log/anytls-client.log"
echo ""
