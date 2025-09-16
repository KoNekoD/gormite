const maxWidth = "90rem";

export function Main(props) {
	return (
		<main
			style={{
				maxWidth,
				margin: "auto",
				...(props.styles ?? {}),
			}}
			{...props}
		/>
	);
}
