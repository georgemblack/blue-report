const API_URL = "https://data.theblue.report/data/top-links.json";

// v2.theblue.report shows the rankings from the new (v2) back-end, which
// publishes under "engine/". Every other host uses the current data.
export const V2_HOST = "v2.theblue.report";
const V2_API_URL = "https://data.theblue.report/engine/data/top-links.json";

export interface TopLinks {
  generated_at: string;
  top_hour: Link[];
  top_day: Link[];
  top_week: Link[];
}

export interface Link {
  rank: number;
  url: string;
  title: string;
  thumbnail_url: string;
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

export async function fetchTopLinks(host: string): Promise<TopLinks> {
  const response = await fetch(host === V2_HOST ? V2_API_URL : API_URL);
  return await response.json();
}

export interface TopSites {
  generated_at: string;
  sites: Site[];
}

export interface Site {
  rank: number;
  name: string;
  domain: string;
  interactions: number;
  links: {
    url: string;
    title: string;
    thumbnail_url: string;
    interactions: number;
  }[];
}

export async function fetchTopSites(): Promise<TopSites> {
  const response = await fetch(
    "https://data.theblue.report/data/top-sites.json"
  );
  return await response.json();
}
