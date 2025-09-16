/* eslint-env node */
import { Layout, Footer, Navbar } from "nextra-theme-docs";
import { Head } from "nextra/components";
import { getPageMap } from "nextra/page-map";
import "nextra-theme-docs/style.css";

export const metadata = {
	metadataBase: new URL("https://github.com/KoNekoD/gormite"),
	title: {
		template: "%s - Gormite",
	},
	description: "Gormite: the next Go ORM",
	applicationName: "Gormite",
	generator: "Next.js",
	appleWebApp: {
		title: "Gormite",
	},
};

export default async function RootLayout({ children }) {
	const navbar = (
		<Navbar
			logo={
				<div>
					<b>Gormite</b> <span style={{ opacity: "60%" }}>The Next Go ORM</span>
				</div>
			}
		/>
	);
	return (
		<html lang="en" dir="ltr" suppressHydrationWarning>
			<Head faviconGlyph="✦" />
			<body
				style={{
					height: "100vh",
				}}
			>
				<Layout
					navbar={navbar}
					footer={<Footer about="Gormit">LGPL-3.0-only © Gormite</Footer>}
					editLink="Edit this page on GitHub"
					docsRepositoryBase="https://github.com/shuding/KoNekoD/blob/main/docs/web"
					sidebar={{ defaultMenuCollapseLevel: 1 }}
					pageMap={await getPageMap()}
				>
					{children}
				</Layout>
			</body>
		</html>
	);
}
