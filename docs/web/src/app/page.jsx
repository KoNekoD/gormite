import { Main } from "../components/main";
import Image from "next/image";
import s from "./page.module.css";
import { Link } from "nextra-theme-docs";

export const metadata = {};

export default function HomePage() {
	return (
		<Main>
			<section className={s.home__container}>
				<div>
					<h1
						style={{
							fontSize: "1.5rem",
						}}
					>
						Gormite
					</h1>
					<p>The Next Go ORM</p>
					<div className={s.home__links}>
						<Link href="/docs">Documentation</Link>
					</div>
				</div>
				<Image width={350} height={350} src="/favicon.ico" alt="logo" />
			</section>
		</Main>
	);
}
