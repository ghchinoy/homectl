import asyncio
import logging
import os
import sys
from pathlib import Path
from pylutron_caseta.pairing import async_pair

# Configure logging
logging.basicConfig(level=logging.INFO)

def get_config_dir():
    # Matches Go's os.UserConfigDir() behavior for Linux
    config_home = os.environ.get("XDG_CONFIG_HOME")
    if config_home:
        return Path(config_home) / "homectl"
    return Path.home() / ".config" / "homectl"

async def pair():
    bridge_ip = sys.argv[1] if len(sys.argv) > 1 else os.environ.get("HOMECTL_LUTRON_BRIDGE", "")
    if not bridge_ip:
        print("Usage: pair_lutron.py <bridge_ip> or set HOMECTL_LUTRON_BRIDGE environment variable", file=sys.stderr)
        return
    config_dir = get_config_dir()
    config_dir.mkdir(parents=True, exist_ok=True)
    
    print(f"Connecting to Lutron Bridge at {bridge_ip}...")
    
    def on_ready():
        print("\n" + "="*50)
        print("BRIDGE IS READY FOR PAIRING!")
        print("ACTION REQUIRED: Press the button on the back of your Lutron Bridge.")
        print("="*50 + "\n")

    try:
        # async_pair takes the IP and an optional callback for when it's ready for the button press
        data = await async_pair(bridge_ip, ready=on_ready)
        
        # The data returned is a dictionary-like object (PairingData)
        # Based on library source, it contains 'ca', 'cert', and 'key'
        ca_path = config_dir / "lutron_ca.crt"
        cert_path = config_dir / "lutron_client.crt"
        key_path = config_dir / "lutron_client.key"

        with open(ca_path, "w") as f:
            f.write(data["ca"])
        with open(cert_path, "w") as f:
            f.write(data["cert"])
        with open(key_path, "w") as f:
            f.write(data["key"])
            
        print(f"\nSUCCESS! Certificates have been saved to {config_dir}:")
        print(f"- {ca_path.name}")
        print(f"- {cert_path.name}")
        print(f"- {key_path.name}")
        
    except Exception as e:
        print(f"\nError during pairing: {e}")

if __name__ == "__main__":
    asyncio.run(pair())