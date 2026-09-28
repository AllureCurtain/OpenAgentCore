"use client"

import Image from "next/image"
import Link from "next/link"
import { useParams, usePathname, useRouter } from "next/navigation"
import { FrameworkProvider, type Framework } from "fumadocs-core/framework"
import { RootProvider } from "fumadocs-ui/provider/base"
import type { ReactNode } from "react"

export function DocsProviders({ children }: { children: ReactNode }) {
  return (
    <FrameworkProvider usePathname={usePathname} useParams={useParams} useRouter={useRouter}
      Link={Link as Framework["Link"]} Image={Image as Framework["Image"]}>
      <RootProvider search={{ enabled: false }}>{children}</RootProvider>
    </FrameworkProvider>
  )
}
