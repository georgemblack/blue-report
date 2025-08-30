import websocket
import json
import re
import time
from datetime import datetime

# Global stats
stats = {
    'messages_processed': 0,
    'posts_processed': 0,
    'youtube_posts_found': 0,
    'other_operations': 0,
    'start_time': None
}

# YouTube URL patterns
YOUTUBE_PATTERNS = [
    r'https://youtube\.com',
    r'https://www\.youtube\.com',
    r'https://youtu\.be',
    r'https://www\.youtu\.be'
]

def extract_youtube_urls(text):
    """Extract YouTube URLs from text"""
    urls = []
    for pattern in YOUTUBE_PATTERNS:
        matches = re.findall(rf'{pattern}[^\s]*', text, re.IGNORECASE)
        urls.extend(matches)
    return urls

def bsky_post_url(evt, handle_or_did=None):
    """Generate Bluesky post URL from event data"""
    if (evt.get('commit', {}).get('collection') != 'app.bsky.feed.post' or 
        evt.get('commit', {}).get('operation') != 'create'):
        return None
    
    rkey = evt.get('commit', {}).get('rkey')
    did = handle_or_did or evt.get('did')
    
    if not rkey or not did:
        return None
        
    return f"https://bsky.app/profile/{did}/post/{rkey}"

def print_stats():
    """Print current statistics"""
    if stats['start_time']:
        runtime = datetime.now() - stats['start_time']
        runtime_str = str(runtime).split('.')[0]  # Remove microseconds
        rate = stats['messages_processed'] / runtime.total_seconds() if runtime.total_seconds() > 0 else 0
        youtube_rate = (stats['youtube_posts_found'] / stats['posts_processed'] * 100) if stats['posts_processed'] > 0 else 0
        print(f"\rTotal: {stats['messages_processed']:,} | "
              f"Posts: {stats['posts_processed']:,} | "
              f"YouTube: {stats['youtube_posts_found']} ({youtube_rate:.2f}%) | "
              f"Other ops: {stats['other_operations']:,} | "
              f"Runtime: {runtime_str} | {rate:.1f} msg/s", end='', flush=True)

def on_message(ws, message):
    try:
        stats['messages_processed'] += 1
        data = json.loads(message)

        # Print stats every 100 messages
        if stats['messages_processed'] % 100 == 0:
            print_stats()

        # Track different message types
        if data.get('kind') == 'commit':
            if (data.get('commit', {}).get('operation') == 'create' and
                data.get('commit', {}).get('collection') == 'app.bsky.feed.post'):
                
                stats['posts_processed'] += 1
                record = data.get('commit', {}).get('record', {})
                text = record.get('text', '')

                # Check for YouTube URLs
                youtube_urls = extract_youtube_urls(text)
                if youtube_urls:
                    stats['youtube_posts_found'] += 1
                    
                    # Create output record
                    output = {
                        'timestamp': datetime.now().isoformat(),
                        'did': data.get('did'),
                        'text': text,
                        'youtube_urls': youtube_urls,
                        'created_at': record.get('createdAt'),
                        'langs': record.get('langs', []),
                        'bsky_url': bsky_post_url(data)
                    }

                    # Write to file
                    with open('youtube_posts.jsonl', 'a', encoding='utf-8') as f:
                        f.write(json.dumps(output) + '\n')

                    print(f"\nFound YouTube post #{stats['youtube_posts_found']}: {youtube_urls}")
                    print_stats()
            else:
                stats['other_operations'] += 1

    except Exception as e:
        print(f"\nError processing message: {e}")

def on_error(ws, error):
    print(f"WebSocket error: {error}")

def on_close(ws, close_status_code, close_msg):
    print(f"WebSocket connection closed (status: {close_status_code})")

def on_open(ws):
    stats['start_time'] = datetime.now()
    stats['messages_processed'] = 0
    stats['posts_processed'] = 0
    stats['youtube_posts_found'] = 0
    stats['other_operations'] = 0
    print("Connected to Bluesky Jetstream")
    print("Listening for YouTube posts...")
    print("Stats: Total msgs | Posts | YouTube (%) | Other ops | Runtime | Rate")

def connect_with_retry():
    """Connect to WebSocket with exponential backoff retry logic"""
    max_retries = 1000
    retry_delay = 1
    max_delay = 300
    
    for attempt in range(max_retries):
        try:
            print(f"Connection attempt {attempt + 1}...")
            
            ws = websocket.WebSocketApp(
                "wss://jetstream2.us-east.bsky.network/subscribe?wantedCollections=app.bsky.feed.post",
                on_open=on_open,
                on_message=on_message,
                on_error=on_error,
                on_close=on_close
            )
            
            ws.run_forever(ping_interval=30, ping_timeout=10)
            
        except KeyboardInterrupt:
            print("\n\nStopping...")
            if stats['start_time']:
                print_stats()
                print("\n")
            break
        except Exception as e:
            print(f"Connection failed: {e}")
            
        if attempt < max_retries - 1:
            print(f"Retrying in {retry_delay} seconds...")
            time.sleep(retry_delay)
            retry_delay = min(retry_delay * 2, max_delay)
        else:
            print("Max retries reached. Exiting.")

if __name__ == "__main__":
    websocket.enableTrace(False)
    connect_with_retry()
