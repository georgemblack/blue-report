# The Blue Report Ranking

## Counting

Each link on The Blue Report displays the number of posts, reposts, and likes for a given link, defined as:

* **Posts:** The number of posts containing the link, as well as the number of posts *quoting* a post that contains the link (i.e. quote posts)
* **Reposts:** The number of reposts of posts that contain the link
* **Likes:** The number of likes of posts that contain the link

The dispalyed number represnts how many posts/reposts/likes have occurred within the previous 24 hours, *not* how many posts/reposts/likes have occurred "overall".

Because of this, it is possible to have a link with:

* 0 posts
* 100 reposts
* 1,000 likes

This simply means that all posts referencing the link occurred more than 24 hours ago. However, users liked and reposted those posts within the last 24 hours.

## Scoring

Links are via their score, calculated by:

```
score = (10 * posts) + (10 * reposts) * (1 * likes)
```

## Disclaimers

**The numbers displayed on The Blue Report should be considered an estimate.** The Blue Report is a small side project, and may contain bugs, or have brief periods of downtime causing missing posts/reposts/likes.