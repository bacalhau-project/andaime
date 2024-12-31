import boto3
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from botocore.exceptions import ClientError
from rich.console import Console
from rich.table import Table
from rich.progress import Progress, SpinnerColumn, TextColumn
from rich.panel import Panel
from rich import box

console = Console()

def get_all_regions():
    """Get all AWS regions"""
    ec2 = boto3.client('ec2')
    response = ec2.describe_regions()
    return [region['RegionName'] for region in response['Regions']]

def find_matching_vpcs(region):
    """Find VPCs with matching tags in a region"""
    try:
        ec2 = boto3.client('ec2', region_name=region)
        response = ec2.describe_vpcs(
            Filters=[
                {'Name': 'tag:Name', 'Values': ['andaime-vpc*']},
                {'Name': 'tag:andaime', 'Values': ['true']}
            ]
        )
        return region, response['Vpcs']
    except Exception as e:
        return region, []

def list_vpc_resources(region, vpc_id):
    """List all resources in a VPC"""
    ec2 = boto3.client('ec2', region_name=region)
    
    resources = {
        'Subnets': [],
        'Internet Gateways': [],
        'Route Tables': [],
        'Security Groups': [],
        'Instances': []
    }
    
    # Get all resources in parallel
    with ThreadPoolExecutor() as executor:
        futures = {
            executor.submit(ec2.describe_subnets, Filters=[{'Name': 'vpc-id', 'Values': [vpc_id]}]): 'Subnets',
            executor.submit(ec2.describe_internet_gateways, Filters=[{'Name': 'attachment.vpc-id', 'Values': [vpc_id]}]): 'Internet Gateways',
            executor.submit(ec2.describe_route_tables, Filters=[{'Name': 'vpc-id', 'Values': [vpc_id]}]): 'Route Tables',
            executor.submit(ec2.describe_security_groups, Filters=[{'Name': 'vpc-id', 'Values': [vpc_id]}]): 'Security Groups',
            executor.submit(ec2.describe_instances, Filters=[{'Name': 'vpc-id', 'Values': [vpc_id]}]): 'Instances'
        }
        
        for future in as_completed(futures):
            resource_type = futures[future]
            try:
                response = future.result()
                if resource_type == 'Subnets':
                    resources[resource_type] = [s['SubnetId'] for s in response['Subnets']]
                elif resource_type == 'Internet Gateways':
                    resources[resource_type] = [ig['InternetGatewayId'] for ig in response['InternetGateways']]
                elif resource_type == 'Route Tables':
                    resources[resource_type] = [rt['RouteTableId'] for rt in response['RouteTables'] 
                                              if not any(assoc['Main'] for assoc in rt['Associations'])]
                elif resource_type == 'Security Groups':
                    resources[resource_type] = [sg['GroupId'] for sg in response['SecurityGroups'] 
                                              if sg['GroupName'] != 'default']
                elif resource_type == 'Instances':
                    resources[resource_type] = [i['InstanceId'] for r in response['Reservations'] 
                                              for i in r['Instances']]
            except Exception as e:
                console.print(f"[red]Error fetching {resource_type}: {e}[/red]")
    
    return resources

def display_vpc_table(vpcs):
    """Display VPCs in a pretty table"""
    table = Table(title="Matching VPCs", box=box.ROUNDED)
    table.add_column("Region", style="cyan")
    table.add_column("VPC ID", style="magenta")
    table.add_column("Name", style="green")
    table.add_column("Tags", style="yellow")
    
    for region, vpc_list in vpcs.items():
        for vpc in vpc_list:
            vpc_id = vpc['VpcId']
            vpc_name = next((tag['Value'] for tag in vpc.get('Tags', []) if tag['Key'] == 'Name'), '')
            tags = ", ".join(f"{tag['Key']}={tag['Value']}" for tag in vpc.get('Tags', []))
            table.add_row(region, vpc_id, vpc_name, tags)
    
    console.print(Panel(table, title="[bold]Discovered VPCs[/bold]", border_style="blue"))

def display_resource_table(resources):
    """Display resources in a pretty table"""
    table = Table(title="VPC Resources", box=box.ROUNDED)
    table.add_column("Resource Type", style="cyan")
    table.add_column("Count", style="magenta")
    table.add_column("IDs", style="yellow")
    
    for resource_type, items in resources.items():
        count = len(items)
        ids = "\n".join(items) if items else "None"
        table.add_row(resource_type, str(count), ids)
    
    console.print(Panel(table, title="[bold]VPC Resources[/bold]", border_style="blue"))

def delete_vpc_resources(region, vpc_id):
    """Delete all resources in a VPC"""
    ec2 = boto3.client('ec2', region_name=region)
    
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        transient=True
    ) as progress:
        # Terminate instances
        task = progress.add_task("[cyan]Terminating instances...", total=1)
        instances = ec2.describe_instances(Filters=[{'Name': 'vpc-id', 'Values': [vpc_id]}])
        instance_ids = [i['InstanceId'] for r in instances['Reservations'] for i in r['Instances']]
        
        if instance_ids:
            ec2.terminate_instances(InstanceIds=instance_ids)
            waiter = ec2.get_waiter('instance_terminated')
            waiter.wait(InstanceIds=instance_ids)
        progress.update(task, completed=1)
        
        # Delete security groups
        task = progress.add_task("[cyan]Deleting security groups...", total=1)
        security_groups = ec2.describe_security_groups(Filters=[{'Name': 'vpc-id', 'Values': [vpc_id]}])
        for sg in security_groups['SecurityGroups']:
            if sg['GroupName'] != 'default':
                ec2.delete_security_group(GroupId=sg['GroupId'])
        progress.update(task, completed=1)
        
        # Delete subnets
        task = progress.add_task("[cyan]Deleting subnets...", total=1)
        subnets = ec2.describe_subnets(Filters=[{'Name': 'vpc-id', 'Values': [vpc_id]}])
        for subnet in subnets['Subnets']:
            ec2.delete_subnet(SubnetId=subnet['SubnetId'])
        progress.update(task, completed=1)
        
        # Detach and delete internet gateways
        task = progress.add_task("[cyan]Deleting internet gateways...", total=1)
        igws = ec2.describe_internet_gateways(Filters=[{'Name': 'attachment.vpc-id', 'Values': [vpc_id]}])
        for igw in igws['InternetGateways']:
            ec2.detach_internet_gateway(InternetGatewayId=igw['InternetGatewayId'], VpcId=vpc_id)
            ec2.delete_internet_gateway(InternetGatewayId=igw['InternetGatewayId'])
        progress.update(task, completed=1)
        
        # Delete route tables
        task = progress.add_task("[cyan]Deleting route tables...", total=1)
        route_tables = ec2.describe_route_tables(Filters=[{'Name': 'vpc-id', 'Values': [vpc_id]}])
        for rt in route_tables['RouteTables']:
            if not any(assoc['Main'] for assoc in rt['Associations']):
                ec2.delete_route_table(RouteTableId=rt['RouteTableId'])
        progress.update(task, completed=1)
        
        # Delete VPC with retries
        task = progress.add_task("[cyan]Deleting VPC...", total=1)
        max_retries = 3
        for attempt in range(max_retries):
            try:
                ec2.delete_vpc(VpcId=vpc_id)
                break
            except ClientError as e:
                if attempt == max_retries - 1:
                    raise
                time.sleep(5)
        progress.update(task, completed=1)

def main():
    console.print(Panel("[bold green]AWS VPC Cleanup Tool[/bold green]", box=box.DOUBLE))
    
    # Get all regions and find VPCs in parallel
    regions = get_all_regions()
    vpcs = {}
    
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        transient=True
    ) as progress:
        task = progress.add_task("[cyan]Searching for VPCs across all regions...", total=len(regions))
        
        with ThreadPoolExecutor() as executor:
            future_to_region = {executor.submit(find_matching_vpcs, region): region for region in regions}
            
            for future in as_completed(future_to_region):
                region, vpc_list = future.result()
                if vpc_list:
                    vpcs[region] = vpc_list
                progress.update(task, advance=1)
    
    if not any(vpcs.values()):
        console.print("[yellow]No matching VPCs found in any region.[/yellow]")
        return
    
    display_vpc_table(vpcs)
    
    # List resources for each VPC
    for region, vpc_list in vpcs.items():
        for vpc in vpc_list:
            vpc_id = vpc['VpcId']
            console.print(f"\n[bold]Listing resources for VPC {vpc_id} in {region}[/bold]")
            resources = list_vpc_resources(region, vpc_id)
            display_resource_table(resources)
    
    # Confirm before deletion
    confirm = console.input("\n[bold red]Are you sure you want to delete all these VPCs and their resources? (y/N): [/]").lower()
    if confirm != 'y':
        console.print("[yellow]Operation cancelled.[/yellow]")
        return
    
    # Delete resources in parallel
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        transient=True
    ) as progress:
        total_vpcs = sum(len(vpc_list) for vpc_list in vpcs.values())
        task = progress.add_task("[red]Deleting VPCs and resources...", total=total_vpcs)
        
        with ThreadPoolExecutor() as executor:
            futures = []
            for region, vpc_list in vpcs.items():
                for vpc in vpc_list:
                    vpc_id = vpc['VpcId']
                    futures.append(executor.submit(delete_vpc_resources, region, vpc_id))
            
            for future in as_completed(futures):
                try:
                    future.result()
                except Exception as e:
                    console.print(f"[red]Error during deletion: {e}[/red]")
                progress.update(task, advance=1)
    
    console.print("[bold green]Cleanup complete![/bold green]")

if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        console.print("\n[yellow]Operation cancelled by user.[/yellow]")
    except Exception as e:
        console.print(f"[red]Error: {e}[/red]")
