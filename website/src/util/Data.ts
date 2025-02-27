const API_URL = "https://assets.theblue.report/snapshot.json";

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

export async function fetchTopLinks(): Promise<TopLinks> {
  const response = await fetch(API_URL);
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
  }[];
}

export async function fetchTopSites(): Promise<TopSites> {
  const response = await fetch("https://assets.theblue.report/sites.json");
  return await response.json();
}
