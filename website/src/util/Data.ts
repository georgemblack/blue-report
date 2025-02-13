const API_URL = "https://assets.theblue.report/snapshot.json";

export interface Snapshot {
  generated_at: string;
  links: Link[];
}

export interface Link {
  rank: number;
  url: string;
  title: string;
  thumbnail_id: string;
  post_count: number;
  repost_count: number;
  like_count: number;
  click_count: number;
  recommended_posts: Post[];
}

export interface Post {
  at_uri: string;
  username: string;
  handle: string;
  text: string;
}

export async function fetchSnapshot(): Promise<Snapshot> {
  const response = await fetch(API_URL);
  const body: Snapshot = await response.json();
  // Keep only the first three of each link's recommended posts
  body.links.forEach((link) => {
    link.recommended_posts = link.recommended_posts.slice(0, 3);
  });

  return body;
}
