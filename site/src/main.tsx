import { hydrate, render } from "preact";
import { pageForPath } from "./pages";
import "./index.css";

const root = document.getElementById("root")!;
const page = pageForPath(window.location.pathname);
if (page) {
  const Page = page.component;
  document.title = page.title;
  if (root.firstChild) hydrate(<Page />, root);
  else render(<Page />, root);
} else {
  render(
    <main>
      <h1>Page not found</h1>
      <a href="/">Return to PRX</a>
    </main>,
    root,
  );
}
