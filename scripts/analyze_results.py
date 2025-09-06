#!/usr/bin/env python3
import re
import json
import statistics
from pathlib import Path
from datetime import datetime

def parse_ping_results(log_file):
    """Parse ping results from log files"""
    results = {
        "successful_pings": 0,
        "failed_pings": 0,
        "rtts": [],
        "errors": [],
        "connection_status": "unknown"
    }
    
    if not Path(log_file).exists():
        print(f"Warning: Log file {log_file} not found")
        return results
    
    with open(log_file, 'r') as f:
        content = f.read()
    
    # Check connection status
    if "Connected to" in content:
        results["connection_status"] = "connected"
    elif "Connection failed" in content or "Failed to connect" in content:
        results["connection_status"] = "failed"
    
    # Extract RTT values - look for various formats (using raw strings)
    rtt_patterns = [
        r'RTT: ([\d.]+)(?:ms)?',
        r'successful - RTT: ([\d.]+)',
        r'ping.*?(\d+\.?\d*)ms'
    ]
    
    all_rtts = []
    for pattern in rtt_patterns:
        matches = re.findall(pattern, content, re.IGNORECASE)
        if matches:
            all_rtts.extend([float(rtt) for rtt in matches])
    
    if all_rtts:
        results["rtts"] = all_rtts
        results["successful_pings"] = len(all_rtts)
    
    # Count successful ping messages
    success_patterns = [
        r'Ping \d+ successful',
        r'successful - RTT:',
        r'ping.*successful'
    ]
    
    total_successful = 0
    for pattern in success_patterns:
        matches = re.findall(pattern, content, re.IGNORECASE)
        total_successful += len(matches)
    
    if total_successful > results["successful_pings"]:
        results["successful_pings"] = total_successful
    
    # Extract errors
    error_patterns = [
        r'failed: (.+)',
        r'Error: (.+)',
        r'Connection failed: (.+)',
        r'timed out'
    ]
    
    for pattern in error_patterns:
        matches = re.findall(pattern, content, re.IGNORECASE)
        results["errors"].extend(matches)
    
    results["failed_pings"] = len(results["errors"])
    
    return results

def parse_server_log(log_file):
    """Parse server log for startup and connection info"""
    info = {
        "peer_id": None,
        "listening_addresses": [],
        "startup_successful": False,
        "incoming_connections": 0
    }
    
    if not Path(log_file).exists():
        return info
    
    with open(log_file, 'r') as f:
        content = f.read()
    
    peer_match = re.search(r'Peer ID: (\w+)', content)
    if peer_match:
        info["peer_id"] = peer_match.group(1)
        info["startup_successful"] = True
    
    addr_matches = re.findall(r'Listening on: (.+)', content)
    info["listening_addresses"] = addr_matches
    
    connection_matches = re.findall(r'connection|connect', content, re.IGNORECASE)
    info["incoming_connections"] = len(connection_matches)
    
    return info

def generate_metrics_report():
    """Generate comprehensive metrics report"""
    results_dir = Path("../results")
    results_dir.mkdir(exist_ok=True)
    
    print("=== Analyzing libp2p Interoperability Test Results ===")
    
    py_client_results = parse_ping_results(results_dir / "py-client-test1.log")
    go_server_info = parse_server_log(results_dir / "go-server.log")
    
    timestamp = datetime.now().isoformat()
    
    report = {
        "test_timestamp": timestamp,
        "test_configuration": {
            "server": "go-libp2p",
            "client": "py-libp2p",
            "protocol": "/ipfs/ping/1.0.0",
            "test_type": "unidirectional"
        },
        "server_info": go_server_info,
        "client_results": py_client_results,
        "overall_stats": {},
        "interoperability_assessment": {}
    }
    
    if py_client_results["rtts"]:
        rtts = py_client_results["rtts"]
        report["overall_stats"] = {
            "total_ping_attempts": 5,  # Expected number of pings
            "successful_pings": py_client_results["successful_pings"],
            "failed_pings": py_client_results["failed_pings"],
            "average_rtt_ms": round(statistics.mean(rtts), 2),
            "min_rtt_ms": round(min(rtts), 2),
            "max_rtt_ms": round(max(rtts), 2),
            "std_dev_rtt_ms": round(statistics.stdev(rtts), 2) if len(rtts) > 1 else 0,
            "connection_established": py_client_results["connection_status"] == "connected"
        }
    else:
        report["overall_stats"] = {
            "total_ping_attempts": 5,
            "successful_pings": 0,
            "failed_pings": py_client_results["failed_pings"],
            "connection_established": py_client_results["connection_status"] == "connected"
        }
    
    successful = report["overall_stats"]["successful_pings"]
    total_attempts = report["overall_stats"]["total_ping_attempts"]
    
    if total_attempts > 0:
        success_rate = (successful / total_attempts) * 100
        report["interoperability_assessment"] = {
            "success_rate_percentage": round(success_rate, 1),
            "interoperability_status": "PASS" if success_rate >= 60 else "FAIL",
            "connection_compatibility": go_server_info["startup_successful"] and py_client_results["connection_status"] == "connected",
            "protocol_compliance": len(py_client_results["rtts"]) > 0,
            "performance_category": get_performance_category(report["overall_stats"].get("average_rtt_ms", 0))
        }
    
    with open(results_dir / "metrics_report.json", 'w') as f:
        json.dump(report, f, indent=2)
    
    display_summary(report)
    
    return report

def get_performance_category(avg_rtt):
    """Categorize performance based on RTT"""
    if avg_rtt == 0:
        return "NO_DATA"
    elif avg_rtt < 10:
        return "EXCELLENT"
    elif avg_rtt < 50:
        return "GOOD"
    elif avg_rtt < 100:
        return "FAIR"
    else:
        return "POOR"

def display_summary(report):
    """Display test results summary"""
    print("\n=== Test Results Summary ===")
    
    server_info = report["server_info"]
    print(f"Go-libp2p Server:")
    print(f"  Startup: {'Success' if server_info['startup_successful'] else '❌ Failed'}")
    if server_info["peer_id"]:
        print(f"  Peer ID: {server_info['peer_id']}")
    print(f"  Listening Addresses: {len(server_info['listening_addresses'])}")
    
    client_results = report["client_results"]
    stats = report["overall_stats"]
    assessment = report["interoperability_assessment"]
    
    print(f"\nPy-libp2p Client:")
    print(f"  Connection: {'Connected' if client_results['connection_status'] == 'connected' else '❌ Failed'}")
    print(f"  Successful Pings: {stats['successful_pings']}/5")
    print(f"  Success Rate: {assessment['success_rate_percentage']}%")
    
    if stats.get("average_rtt_ms", 0) > 0:
        print(f"  Average RTT: {stats['average_rtt_ms']}ms")
        print(f"  RTT Range: {stats['min_rtt_ms']}ms - {stats['max_rtt_ms']}ms")
        print(f"  Performance: {assessment['performance_category']}")
    
    print(f"\n=== Interoperability Assessment ===")
    status = assessment['interoperability_status']
    print(f"Overall Status: {'PASS' if status == 'PASS' else 'FAIL'}")
    print(f"Protocol Compliance: {'Yes' if assessment['protocol_compliance'] else 'No'}")
    print(f"Connection Compatibility: {'Yes' if assessment['connection_compatibility'] else 'No'}")

    # Errors (if any)
    if client_results["errors"]:
        print(f"\n=== Errors Encountered ===")
        for error in client_results["errors"][:5]:
            print(f"  • {error}")
    
    print(f"\nDetailed report saved to: results/metrics_report.json")
    print(f"Test completed at: {report['test_timestamp']}")

if __name__ == "__main__":
    try:
        generate_metrics_report()
    except Exception as e:
        print(f"Error analyzing results: {e}")
        print("Please ensure test logs are present in the results/ directory")
