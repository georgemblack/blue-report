import puppeteer from '@cloudflare/puppeteer';

export default {
	async fetch(request, env, ctx): Promise<Response> {
		const { searchParams } = new URL(request.url);
		let url = searchParams.get('url');
		if (!url) return new Response(null, { status: 400 });
		const browser = await puppeteer.launch(env.BLUE_REPORT_BROWSER);
		const page = await browser.newPage();
		await page.goto(url);
		return new Response(await page.content(), { status: 200 });
	},
} satisfies ExportedHandler<Env>;
