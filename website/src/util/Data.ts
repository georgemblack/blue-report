const API_URL = "https://theblue.report/snapshot.json";

interface Snapshot {
  generated_at: string;
  links: Link[];
}

interface Link {
  rank: number;
  url: string;
  title: string;
  thumbnail_id: string;
  aggregation: Aggregation;
}

interface Aggregation {
  posts: number;
  reposts: number;
  likes: number;
  clicks: number;
}

export async function fetchSnapshot(): Promise<Snapshot> {
  const response = await fetch(API_URL);
  return response.json();
}
