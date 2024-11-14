import subprocess
import requests
import time
import mysql.connector
from datetime import datetime
import logging
import platform
import sys
import json


class TiupChecker:
    def __init__(self):
        self.logger = self._setup_logger()
        self.api_endpoint = "http://localhost:5050/status"
        self.errors = []
        self.platform_info = self._get_platform_info()
        self.component_versions = {}
    
    def _get_platform_info(self):
        """Get platform and architecture information"""
        system = platform.system().lower()  # darwin or linux
        machine = platform.machine().lower()
        
        # Normalize architecture name
        arch = "amd64" if machine in ["x86_64", "amd64"] else "arm64"
        
        return {
            "os": system,
            "arch": arch,
            "platform": f"{system}-{arch}"
        }

    def _setup_logger(self):
        """Setup logger with both file and console handlers"""
        logger = logging.getLogger(__name__)
        logger.setLevel(logging.DEBUG)

        # Console handler
        console_handler = logging.StreamHandler()
        console_handler.setLevel(logging.INFO)
        console_format = logging.Formatter(
            '%(asctime)s [%(levelname)s] %(message)s',
            datefmt='%Y-%m-%d %H:%M:%S'
        )
        console_handler.setFormatter(console_format)

        # File handler
        file_handler = logging.FileHandler('tiup_checker.log')
        file_handler.setLevel(logging.DEBUG)
        file_format = logging.Formatter(
            '%(asctime)s [%(levelname)s] [%(filename)s:%(lineno)d] - %(message)s',
            datefmt='%Y-%m-%d %H:%M:%S'
        )
        file_handler.setFormatter(file_format)

        logger.addHandler(console_handler)
        logger.addHandler(file_handler)
        return logger

    def _record_error(self, stage, error_msg):
        """Record error information"""
        self.errors.append({
            "stage": stage,
            "error": str(error_msg),
            "timestamp": datetime.now().isoformat()
        })
        self.logger.error(f"{stage}: {error_msg}")

    def check_tiup_download(self):
        """Check TiUP artifact download"""
        components = [
            "tidb:nightly",
            "tikv:nightly",
            "pd:nightly",
            "tiflash:nightly",
            "prometheus:nightly",
            "grafana:nightly"
        ]
        
        try:
            # First update tiup itself
            result = subprocess.run(
                ["tiup", "update", "--self"],
                capture_output=True,
                text=True,
                check=True
            )
            self.logger.info("TiUP self-update successful")
            
            # Then install each component
            for component in components:
                self.logger.info(f"Installing component: {component}")
                result = subprocess.run(
                    ["tiup", "install", component],
                    capture_output=True,
                    text=True,
                    check=True
                )
                self.logger.info(f"Component {component} installed successfully")
                
            return True
            
        except subprocess.CalledProcessError as e:
            self._record_error("download", f"Update/installation failed: {e.stderr}")
            return False

    def start_playground(self):
        """Start TiUP playground and wait for it to be ready"""
        self.logger.info("Starting TiUP playground...")
        try:
            cmd = ["tiup", "playground", "nightly"]
            self.logger.debug(f"Executing command: {' '.join(cmd)}")
            
            process = subprocess.Popen(
                cmd,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE
            )
            
            self.logger.info("Waiting initial 10 seconds for startup...")
            time.sleep(10)
            
            # Check port 4000 readiness, max 12 times (2 minutes), 10s interval
            for attempt in range(12):
                self.logger.debug(f"Connection attempt {attempt + 1}/12")
                try:
                    conn = mysql.connector.connect(
                        host="127.0.0.1",
                        user="root",
                        port=4000,
                        connection_timeout=5
                    )
                    conn.close()
                    self.logger.info("TiUP playground started successfully, port 4000 is ready")
                    return process
                except Exception as e:
                    self.logger.debug(f"Connection attempt failed: {str(e)}")
                    
                    # Check if process has exited unexpectedly
                    if process.poll() is not None:
                        stderr = process.stderr.read().decode()
                        self.logger.error(f"Process exited unexpectedly: {stderr}")
                        self._record_error("playground", f"Startup failed: {stderr}")
                        return None
                        
                    self.logger.debug("Waiting 10 seconds before next attempt...")
                    time.sleep(10)
            
            # If we reach here, startup timed out
            self.logger.error("Startup timeout: port 4000 not ready after 2 minutes")
            self._record_error("playground", "Startup timeout: port 4000 not ready after 2 minutes")
            process.terminate()
            return None
                
        except Exception as e:
            self.logger.error(f"Unexpected error during playground startup: {str(e)}")
            self._record_error("playground", f"Startup exception: {str(e)}")
            return None

    def run_smoke_test(self):
        """Run smoke tests against TiDB cluster"""
        self.logger.info("Starting smoke tests...")
        try:
            self.logger.debug("Attempting to connect to TiDB...")
            conn = mysql.connector.connect(
                host="127.0.0.1",
                user="root",
                port=4000,
                connection_timeout=10
            )
            cursor = conn.cursor()

            test_steps = [
                ("Create database", "CREATE DATABASE IF NOT EXISTS test"),
                ("Select database", "USE test"),
                ("Create table", "CREATE TABLE IF NOT EXISTS smoke_test (id INT PRIMARY KEY, value VARCHAR(255))"),
                ("Insert data", "INSERT INTO smoke_test VALUES (1, 'test')"),
                ("Query data", "SELECT * FROM smoke_test"),
            ]

            # Execute basic test steps
            for step_name, sql in test_steps:
                self.logger.debug(f"Executing test step: {step_name} - SQL: {sql}")
                try:
                    cursor.execute(sql)
                    if "SELECT" in sql:
                        result = cursor.fetchall()
                        self.logger.debug(f"Query result: {result}")
                except Exception as e:
                    self.logger.error(f"Test step '{step_name}' failed: {str(e)}")
                    self._record_error("smoke_test", f"{step_name} failed: {str(e)}")
                    return False

            # Add version consistency check
            try:
                cursor.execute("SELECT * FROM information_schema.cluster_info")
                cluster_info = cursor.fetchall()
                self.logger.debug(f"Cluster info: {cluster_info}")
                
                # Extract version info for all components
                self.component_versions = {}  # Reset component versions
                for row in cluster_info:
                    component = row[0]    # Component name
                    version = row[3]      # Version string
                    git_hash = row[4]     # Git commit SHA

                    if component not in ['tidb', 'pd', 'tikv', 'tiflash']:
                        continue

                    # Verify git hash format
                    if not git_hash or len(git_hash) != 40:
                        error_msg = f"Invalid git hash for {component}: {git_hash}"
                        self.logger.error(error_msg)
                        self._record_error("version_check", error_msg)
                        return False

                    # Extract base version (e.g., "8.5.0-alpha" from "8.5.0-alpha-74-g1770006c2e")
                    base_version = version.split('-')[0:2]  # Take first two parts
                    base_version = '-'.join(base_version)
                    self.component_versions[component] = {
                        'full_version': version,
                        'base_version': base_version,
                        'git_hash': git_hash
                    }

                # Check if we found any components
                if not self.component_versions:
                    error_msg = "No valid components found in cluster_info"
                    self.logger.error(error_msg)
                    self._record_error("version_check", error_msg)
                    return False

                # Get the first component's version as reference
                reference_version = next(iter(self.component_versions.values()))['base_version']
                
                # Check version consistency across all components
                for component, info in self.component_versions.items():
                    if info['base_version'] != reference_version:
                        error_msg = (f"Version mismatch: {component} has version {info['base_version']}, "
                                   f"expected {reference_version} (like other components)")
                        self.logger.error(error_msg)
                        self._record_error("version_check", error_msg)
                        return False

                self.logger.info(f"Version check completed successfully. All components at version: {reference_version}")
                self.logger.debug("Component details:")
                for component, info in self.component_versions.items():
                    self.logger.debug(f"{component}: {info['full_version']} ({info['git_hash']})")

            except Exception as e:
                self.logger.error(f"Version check failed: {str(e)}")
                self._record_error("version_check", f"Version check failed: {str(e)}")
                return False

            cursor.close()
            conn.close()
            self.logger.info("Smoke tests completed successfully")
            return True
        except Exception as e:
            self.logger.error(f"Database connection failed: {str(e)}")
            self._record_error("smoke_test", f"Database connection failed: {str(e)}")
            return False

    def send_report(self, status):
        """Send check report to remote server"""
        payload = {
            "timestamp": datetime.now().isoformat(),
            "status": status,
            "platform": self.platform_info["platform"],
            "os": self.platform_info["os"],
            "arch": self.platform_info["arch"],
            "errors": self.errors if self.errors else None,
            "version": {
                "tiup": self._get_tiup_version(),
                "python": sys.version,
                "components": self.component_versions
            }
        }

        try:
            print(json.dumps(payload, indent=2))
            response = requests.post(self.api_endpoint, json=payload)
            response.raise_for_status()
            self.logger.info("Check report sent successfully")
            self.logger.debug(f"Report payload: {json.dumps(payload)}")
            return True
        except requests.exceptions.RequestException as e:
            self.logger.error(f"Failed to send report: {e}")
            return False

    def _get_tiup_version(self):
        """Get TiUP version information"""
        try:
            result = subprocess.run(
                ["tiup", "--version"],
                capture_output=True,
                text=True,
                check=True
            )
            return result.stdout.strip()
        except:
            return "unknown"

    def run_check(self):
        """Run the complete check workflow"""
        self.logger.info("=== Starting TiUP Checker ===")
        self.logger.info(f"Platform info: {self.platform_info}")
        
        status = "success"
        playground_process = None  # Initialize process variable

        if not self.check_tiup_download():
            self.logger.error("TiUP artifact download failed")
            status = "failed"
            return False

        try:
            self.logger.info("Starting playground check...")
            playground_process = self.start_playground()
            if playground_process is None:
                self.logger.error("Playground startup failed")
                status = "failed"
            else:
                self.logger.info("Running smoke tests...")
                if not self.run_smoke_test():
                    self.logger.error("Smoke tests failed")
                    status = "failed"
        finally:
            # Clean up process only if it was started successfully
            if playground_process is not None:
                self.logger.info("Cleaning up playground process...")
                try:
                    playground_process.terminate()
                    playground_process.wait(timeout=30)  # Wait up to 30 seconds for process to terminate
                    self.logger.info("Playground process terminated successfully")
                except subprocess.TimeoutExpired:
                    self.logger.warning("Process termination timed out, forcing kill...")
                    playground_process.kill()  # Force kill if terminate doesn't work
                    self.logger.info("Playground process forcefully killed")
                except Exception as e:
                    self.logger.error(f"Error during process cleanup: {str(e)}")
        
        self.logger.info(f"=== Check completed with status: {status} ===")

        self.send_report(status)
        return status == "success"


if __name__ == "__main__":
    checker = TiupChecker()
    success = checker.run_check()
    print("Check completed, result:", "Success" if success else "Failed")
