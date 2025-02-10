const API_URL = "https://assets.theblue.report/snapshot.json";

interface Snapshot {
  generated_at: string;
  links: Link[];
}

interface Link {
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

interface Post {
  at_uri: string;
  username: string;
  handle: string;
  text: string;
}

export async function fetchSnapshot(): Promise<Snapshot> {
  const response = await fetch(API_URL);
  return await response.json();
}
