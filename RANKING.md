# The Blue Report Ranking

## Ranking Links

'The Blue Report' ranks links based on score, using the following formula:

```
score = (10 * posts) + (10 * reposts) * (1 * likes)
```

Where:

* `posts` is the number of posts either containing the link, or quoting a post that contains the link (i.e. a "quote post")
* `reposts` is the number of reposts of a post that contains the link
* `likes` is the number of likes of a post that contains the link

The following rules also apply:

* Only English language posts are counted
* Only posts/reposts/likes that have occurred within the last 24 hours are counted in the score
* Only one post/repost/like is counted per user, per link
  * Example: if a user likes five separate posts containing the same link, this is counted as one like
* Removal of posts/reposts/likes is not counted
  * Exampe: if a user likes a post containing a link, then removes the like, this is counted as one like

## Displayed Information

The 'posts/reposts/likes' displayed under each link represents the **overall** number of each for the given link, with the following caveats:

* Duplicate events per user are not checked
  * Example: if a user likes five posts containing the same link, this is counted as five likes
  * Example: if a user likes and un-likes the same post five times, this is counted as five likes

The 'clicks' displayed under each link represents how many times users have clicked the given link from The Blue Report's website.

## Disclaimers

**The numbers displayed on The Blue Report should be considered an estimate.** The Blue Report is a small side project, and may contain bugs, or have brief outages/downtime where some posts/reposts/likes are missed.

Questions? Send a message to [contact@george.black](mailto:contact@george.black)!
