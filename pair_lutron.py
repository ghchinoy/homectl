import asyncio
import logging
from pylutron_caseta.pairing import async_pair

# Configure logging
logging.basicConfig(level=logging.INFO)

async def pair():
    bridge_ip = "192.168.4.90"
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
        with open("lutron_ca.crt", "w") as f:
            f.write(data["ca"])
        with open("lutron_client.crt", "w") as f:
            f.write(data["cert"])
        with open("lutron_client.key", "w") as f:
            f.write(data["key"])
            
        print("\nSUCCESS! Certificates have been saved:")
        print("- lutron_ca.crt")
        print("- lutron_client.crt")
        print("- lutron_client.key")
        
    except Exception as e:
        print(f"\nError during pairing: {e}")

if __name__ == "__main__":
    asyncio.run(pair())