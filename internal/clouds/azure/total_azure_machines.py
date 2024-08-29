import json
import subprocess
from collections import defaultdict


def run_azure_cli_command(command):
    result = subprocess.run(command, shell=True, capture_output=True, text=True)
    if result.returncode != 0:
        print(f"Error running command: {command}")
        print(result.stderr)
        return None
    return json.loads(result.stdout)


def get_locations():
    return run_azure_cli_command("az account list-locations --query '[].name' -o json")


def get_vm_sizes(location):
    return run_azure_cli_command(
        f"az vm list-sizes --location {location} --query '[].name' -o json"
    )


def main():
    locations = get_locations()
    if not locations:
        print("Failed to retrieve locations.")
        return

    vm_sizes_count = defaultdict(int)
    vm_sizes_locations = defaultdict(list)

    for location in locations:
        print(f"Processing location: {location}")
        vm_sizes = get_vm_sizes(location)
        if vm_sizes:
            for size in vm_sizes:
                vm_sizes_count[size] += 1
                vm_sizes_locations[size].append(location)

    # Prepare data for tabulation
    table_data = []
    for size, count in sorted(vm_sizes_count.items(), key=lambda x: x[1], reverse=True):
        locations_str = ", ".join(vm_sizes_locations[size])
        table_data.append([size, count, locations_str])

    # Print the table
    headers = ["VM Size", "Count", "Locations"]
    print(tabulate(table_data, headers=headers, tablefmt="grid"))

    # Print summary
    print(f"\nTotal unique VM sizes: {len(vm_sizes_count)}")
    print(f"Total locations: {len(locations)}")


if __name__ == "__main__":
    main()
