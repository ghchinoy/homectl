
import socket

def discover_ssdp():
    msg = [
        'M-SEARCH * HTTP/1.1',
        'HOST: 239.255.255.250:1900',
        'MAN: "ssdp:discover"',
        'MX: 2',
        'ST: ssdp:all',
        '', ''
    ]
    
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM, socket.IPPROTO_UDP)
    sock.settimeout(5)
    
    print("Sending SSDP M-SEARCH...")
    sock.sendto('\r\n'.join(msg).encode('ascii'), ('239.255.255.250', 1900))
    
    devices = set()
    try:
        while True:
            data, addr = sock.recvfrom(65507)
            devices.add(f"{addr[0]}: {data.decode('utf-8', errors='ignore').splitlines()[0]}")
    except socket.timeout:
        pass
    
    print(f"\nFound {len(devices)} unique SSDP responses:")
    for d in sorted(devices):
        print(d)

if __name__ == "__main__":
    discover_ssdp()
