import Link from "next/link"

export default function NotFound() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-4 px-4 py-24 text-center">
      <h1 className="text-3xl font-semibold">Page not found</h1>
      <p className="text-fd-muted-foreground">The requested documentation page does not exist.</p>
      <Link href="/">Back to the manual</Link>
    </main>
  )
}
