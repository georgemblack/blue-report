#!/usr/bin/env python3
import json
import re
from collections import Counter

def extract_youtube_video_ids(text):
    """Extract YouTube video IDs from text using regex patterns."""
    video_ids = []
    
    # Pattern for youtube.com?v= format
    pattern1 = r'youtube\.com.*[?&]v=([a-zA-Z0-9_-]{11})'
    
    # Pattern for youtu.be/ format  
    pattern2 = r'youtu\.be/([a-zA-Z0-9_-]{11})'
    
    # Find matches for both patterns
    matches1 = re.findall(pattern1, text)
    matches2 = re.findall(pattern2, text)
    
    video_ids.extend(matches1)
    video_ids.extend(matches2)
    
    return video_ids

def main():
    video_id_counter = Counter()
    
    try:
        with open('youtube_posts.jsonl', 'r', encoding='utf-8') as f:
            for line in f:
                if line.strip():
                    post = json.loads(line)
                    
                    # Extract video IDs from youtube_urls field
                    if 'youtube_urls' in post:
                        for url in post['youtube_urls']:
                            video_ids = extract_youtube_video_ids(url)
                            for video_id in video_ids:
                                video_id_counter[video_id] += 1
                    
                    # Also check text field for any missed URLs
                    if 'text' in post:
                        video_ids = extract_youtube_video_ids(post['text'])
                        for video_id in video_ids:
                            video_id_counter[video_id] += 1
    
    except FileNotFoundError:
        print("Error: youtube_posts.jsonl not found")
        return
    except json.JSONDecodeError as e:
        print(f"Error parsing JSON: {e}")
        return
    
    # Sort by video ID alphabetically
    sorted_video_ids = sorted(video_id_counter.items())
    
    print("YouTube Video ID Counts:")
    print("========================")
    for video_id, count in sorted_video_ids:
        print(f"{video_id}: {count}")
    
    print(f"\nTotal unique video IDs: {len(sorted_video_ids)}")
    print(f"Total occurrences: {sum(video_id_counter.values())}")

if __name__ == "__main__":
    main()