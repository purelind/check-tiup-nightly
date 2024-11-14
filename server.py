from flask import Flask, request, jsonify
import sqlite3
from datetime import datetime, timedelta
import json

app = Flask(__name__)

def init_db():
    """Initialize database"""
    conn = sqlite3.connect('tiup_checks.db')
    c = conn.cursor()
    c.execute('''
        CREATE TABLE IF NOT EXISTS check_results (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            timestamp TEXT NOT NULL,
            status TEXT NOT NULL,
            platform TEXT NOT NULL,
            os TEXT NOT NULL,
            arch TEXT NOT NULL,
            errors TEXT,
            tiup_version TEXT,
            python_version TEXT,
            components_info TEXT
        )
    ''')
    conn.commit()
    conn.close()

init_db()

@app.route('/status', methods=['POST'])
def report_status():
    """Receive check report"""
    data = request.json
    
    conn = sqlite3.connect('tiup_checks.db')
    c = conn.cursor()
    
    try:
        c.execute('''
            INSERT INTO check_results 
            (timestamp, status, platform, os, arch, errors, tiup_version, python_version, components_info)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        ''', (
            data['timestamp'],
            data['status'],
            data['platform'],
            data['os'],
            data['arch'],
            json.dumps(data['errors']) if data['errors'] else None,
            data['version']['tiup'],
            data['version']['python'],
            json.dumps(data['version'].get('components')) if data['version'].get('components') else None
        ))
        
        conn.commit()
        return jsonify({"status": "success"}), 200
    except Exception as e:
        return jsonify({"status": "error", "message": str(e)}), 500
    finally:
        conn.close()

@app.route('/results', methods=['GET'])
def get_results():
    """Query check results
    - With platform parameter: returns the latest N results for the specified platform
    - Without platform parameter: returns the latest result for each platform
    """
    platform = request.args.get('platform')
    limit = request.args.get('limit', default=10, type=int)  # Default to return 10 records
    
    conn = sqlite3.connect('tiup_checks.db')
    c = conn.cursor()
    
    try:
        # Define a list of valid platforms
        VALID_PLATFORMS = ('linux-amd64', 'linux-arm64', 'darwin-amd64', 'darwin-arm64')
        
        if platform:
            # Verify platform parameter
            if platform not in VALID_PLATFORMS:
                return jsonify({"status": "error", "message": "Invalid platform"}), 400
                
            # Query the latest N results for the specified platform
            c.execute('''
                SELECT * FROM check_results 
                WHERE platform = ?
                ORDER BY timestamp DESC LIMIT ?
            ''', (platform, limit))
        else:
            # Query the latest results for each valid platform
            c.execute('''
                WITH RankedResults AS (
                    SELECT *,
                           ROW_NUMBER() OVER (PARTITION BY platform ORDER BY timestamp DESC) as rn
                    FROM check_results
                    WHERE platform IN (?, ?, ?, ?)
                )
                SELECT id, timestamp, status, platform, os, arch, errors, 
                       tiup_version, python_version, components_info
                FROM RankedResults
                WHERE rn = 1
            ''', VALID_PLATFORMS)
        
        columns = [description[0] for description in c.description]
        results = [dict(zip(columns, row)) for row in c.fetchall()]
        
        # Parse JSON error information stored in the database
        for result in results:
            if result.get('errors'):
                result['errors'] = json.loads(result['errors'])
        
        return jsonify(results), 200
    except Exception as e:
        return jsonify({"status": "error", "message": str(e)}), 500
    finally:
        conn.close()

@app.route('/platforms/<platform>/history', methods=['GET'])
def get_platform_history(platform):
    """Query historical check results for a specific platform
    Parameters:
    - platform: platform identifier
    - days: number of days to query, defaults to 1
    """
    days = request.args.get('days', default=1, type=int)
    
    conn = sqlite3.connect('tiup_checks.db')
    c = conn.cursor()
    
    try:
        # Verify platform parameter
        VALID_PLATFORMS = ('linux-amd64', 'linux-arm64', 'darwin-amd64', 'darwin-arm64')
        if platform not in VALID_PLATFORMS:
            return jsonify({"status": "error", "message": "Invalid platform"}), 400
        
        # Calculate time range
        current_time = datetime.now()
        days_ago = current_time - timedelta(days=days)
        print(days_ago)
        
        # Query check results within the specified time range
        c.execute('''
            SELECT * FROM check_results 
            WHERE platform = ?
            AND timestamp >= ?
            ORDER BY timestamp DESC
        ''', (platform, days_ago.isoformat()))
        
        columns = [description[0] for description in c.description]
        results = [dict(zip(columns, row)) for row in c.fetchall()]
        
        # Parse JSON error information stored in the database
        for result in results:
            if result.get('errors'):
                result['errors'] = json.loads(result['errors'])
        
        return jsonify({
            "status": "success",
            "platform": platform,
            "days": days,
            "total": len(results),
            "results": results
        }), 200
    except Exception as e:
        return jsonify({"status": "error", "message": str(e)}), 500
    finally:
        conn.close()

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5050)