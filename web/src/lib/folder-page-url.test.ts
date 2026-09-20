import { describe, expect, it } from "vitest"
import { folderPageURL } from "./folder-page-url"

describe("folderPageURL", () => {
  it("encodes sort and omits a stale cursor on page one", () => {
    expect(folderPageURL(12, "name_desc")).toBe("/api/folders/12?sort=name_desc")
    expect(folderPageURL(12, "oldest", null)).not.toContain("cursor")
  })

  it("encodes an opaque load-more cursor", () => {
    expect(folderPageURL(12, "latest", "a+b/c?=")).toBe("/api/folders/12?sort=latest&cursor=a%2Bb%2Fc%3F%3D")
  })
})
