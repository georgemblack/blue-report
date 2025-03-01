# The Blue Report Ranking

## How Are Top Links Ranked?

The [main page](https://theblue.report) of The Blue Report displays the top links on Bluesky over the last hour, day, and week. Links are ranked based on score ranks links based on score, using the following formula:

```
score = (10 * posts) + (10 * reposts) * (1 * likes)
```

Where:

* `posts` is the number of posts either containing the link, or quoting a post that contains the link (i.e. a "quote post")
* `reposts` is the number of reposts of a post that contains the link
* `likes` is the number of likes of a post that contains the link

The following rules also apply:

* Only English language posts are counted
* Only posts/reposts/likes that have occurred within the time window are counted in the score
* Only one post/repost/like is counted per user, per link
  * Example: if a user likes five separate posts containing the same link, this is counted as one like
* Removal of posts/reposts/likes is not counted
  * Exampe: if a user likes a post containing a link, then removes the like, this is counted as one like

The 'posts/reposts/likes' displayed under each link represents the number of each that have occurred in the past week, with the same caveats as above.

## How Are Top Sites Ranked?

The 'Top Sites' page of The Blue Report displays the top domains on Bluesky over the last 30 days, based on the number of **interactions**.

**Interactions** refer to the number of posts referencing a URL, combined with the number of reposts & likes on those posts.

The following rules also apply:

* Only one post/repost/like is counted per user, per link
  * This means a single user can only have up to three interactions with a single URL

## Disclaimers

**The numbers displayed on The Blue Report should be considered an estimate.** The Blue Report is a small side project, and may contain bugs, or have brief outages/downtime where some posts/reposts/likes are missed.

Questions? Send a message to [contact@george.black](mailto:contact@george.black)!
