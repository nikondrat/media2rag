import Foundation

final class MmapFileReader {
    private let url: URL
    private var fileHandle: FileHandle?
    private var mappedData: Data?
    private var sectionCache: [Int: String] = [:]
    private var useFallback = false

    init(url: URL) {
        self.url = url
    }

    deinit {
        close()
    }

    func open() throws {
        let fh = try FileHandle(forReadingFrom: url)
        let fileSize = try fh.seekToEnd()
        _ = fileSize
        fh.seek(toFileOffset: 0)

        if let data = try? Data(contentsOf: url) {
            fileHandle = fh
            mappedData = data
            useFallback = false
        } else {
            fileHandle = fh
            useFallback = true
        }
    }

    func close() {
        sectionCache.removeAll()
        mappedData = nil
        try? fileHandle?.close()
        fileHandle = nil
    }

    func scanSections() -> [SectionIndex] {
        guard let data = mappedData, !useFallback else {
            print("[MmapFileReader] scanSections: using fallback, useFallback=\(useFallback), mappedData=\(mappedData != nil ? "yes" : "no")")
            return scanSectionsFallback()
        }

        print("[MmapFileReader] scanSections: starting, data size=\(data.count)")
        var sections: [SectionIndex] = []
        var inFrontmatter = false
        var currentOffset: Int?
        var currentLevel = 0
        var currentTitle = ""

        guard let content = String(data: data, encoding: .utf8) else {
            print("[MmapFileReader] scanSections: failed to decode content as UTF8")
            return []
        }
        let lines = content.split(separator: "\n", omittingEmptySubsequences: false)
        print("[MmapFileReader] scanSections: total lines=\(lines.count)")

        var lineOffset = 0
        for line in lines {
            let trimmed = line.trimmingCharacters(in: .whitespaces)

            if trimmed == "---" {
                if sections.isEmpty && !inFrontmatter {
                    inFrontmatter = true
                    lineOffset += line.count + 1
                    continue
                } else if inFrontmatter {
                    inFrontmatter = false
                    lineOffset += line.count + 1
                    continue
                }
            }

            if inFrontmatter {
                lineOffset += line.count + 1
                continue
            }

            if let idx = trimmed.firstIndex(of: "#"), idx == trimmed.startIndex {
                let hashCount = trimmed.prefix { $0 == "#" }.count
                if hashCount <= 6 {
                    if let prevOffset = currentOffset {
                        let length = lineOffset - prevOffset
                        if length > 0 {
                            sections.append(SectionIndex(
                                id: sections.count,
                                title: currentTitle,
                                offset: prevOffset,
                                length: length,
                                level: currentLevel
                            ))
                        }
                    }
                    let title = String(trimmed.dropFirst(hashCount)).trimmingCharacters(in: .whitespaces)
                    currentOffset = lineOffset
                    currentLevel = Int(hashCount)
                    currentTitle = title
                }
            }

            lineOffset += line.count + 1
        }

        print("[MmapFileReader] scanSections: found \(sections.count) sections")
        if let prevOffset = currentOffset {
            let length = lineOffset - prevOffset
            if length > 0 {
                sections.append(SectionIndex(
                    id: sections.count,
                    title: currentTitle,
                    offset: prevOffset,
                    length: length,
                    level: currentLevel
                ))
                print("[MmapFileReader] scanSections: added final section, id=\(sections.count - 1)")
            }
        }

        return sections
    }

    private func scanSectionsFallback() -> [SectionIndex] {
        guard let fileHandle = fileHandle else { return [] }
        fileHandle.seek(toFileOffset: 0)
        guard let content = try? String(contentsOf: url, encoding: .utf8) else { return [] }

        print("[MmapFileReader] scanSectionsFallback: starting")
        var sections: [SectionIndex] = []
        var inFrontmatter = false
        var currentOffset: Int?
        var currentLevel = 0
        var currentTitle = ""

        let lines = content.split(separator: "\n", omittingEmptySubsequences: false)
        var lineOffset = 0

        for line in lines {
            let trimmed = line.trimmingCharacters(in: .whitespaces)

            if trimmed == "---" {
                if sections.isEmpty && !inFrontmatter {
                    inFrontmatter = true
                    lineOffset += line.count + 1
                    continue
                } else if inFrontmatter {
                    inFrontmatter = false
                    lineOffset += line.count + 1
                    continue
                }
            }

            if inFrontmatter {
                lineOffset += line.count + 1
                continue
            }

            if let idx = trimmed.firstIndex(of: "#"), idx == trimmed.startIndex {
                let hashCount = trimmed.prefix { $0 == "#" }.count
                if hashCount <= 6 {
                    if let prevOffset = currentOffset {
                        let length = lineOffset - prevOffset
                        if length > 0 {
                            sections.append(SectionIndex(
                                id: sections.count,
                                title: currentTitle,
                                offset: prevOffset,
                                length: length,
                                level: currentLevel
                            ))
                        }
                    }
                    let title = String(trimmed.dropFirst(hashCount)).trimmingCharacters(in: .whitespaces)
                    currentOffset = lineOffset
                    currentLevel = Int(hashCount)
                    currentTitle = title
                }
            }

            lineOffset += line.count + 1
        }

        print("[MmapFileReader] scanSectionsFallback: found \(sections.count) sections")
        if let prevOffset = currentOffset {
            let length = lineOffset - prevOffset
            if length > 0 {
                sections.append(SectionIndex(
                    id: sections.count,
                    title: currentTitle,
                    offset: prevOffset,
                    length: length,
                    level: currentLevel
                ))
                print("[MmapFileReader] scanSectionsFallback: added final section, id=\(sections.count - 1)")
            }
        }

        return sections
    }

    func getSectionContent(index: Int, sections: [SectionIndex]) -> String? {
        guard index >= 0 && index < sections.count else {
            print("[MmapFileReader] getSectionContent: index out of range, index=\(index), sections.count=\(sections.count)")
            return nil
        }

        if let cached = sectionCache[index] {
            print("[MmapFileReader] getSectionContent: cache hit, index=\(index), cached length=\(cached.count)")
            return cached
        }

        let section = sections[index]
        print("[MmapFileReader] getSectionContent: cache miss, index=\(index), offset=\(section.offset), length=\(section.length)")

        if useFallback || mappedData == nil {
            return getSectionContentFallback(section)
        }

        guard let data = mappedData else {
            print("[MmapFileReader] getSectionContent: mappedData is nil")
            return nil
        }
        let endOffset = min(section.offset + section.length, data.count)
        guard section.offset < endOffset else {
            print("[MmapFileReader] getSectionContent: invalid offset range, section.offset=\(section.offset), endOffset=\(endOffset)")
            return nil
        }

        let sectionData = data.subdata(in: section.offset..<endOffset)
        if let content = String(data: sectionData, encoding: .utf8) {
            sectionCache[index] = content
            print("[MmapFileReader] getSectionContent: success, index=\(index), content length=\(content.count)")
            return content
        }

        print("[MmapFileReader] getSectionContent: failed to decode section as UTF8")
        return nil
    }

    private func getSectionContentFallback(_ section: SectionIndex) -> String? {
        guard let fileHandle = fileHandle else {
            print("[MmapFileReader] getSectionContentFallback: fileHandle is nil")
            return nil
        }
        do {
            try fileHandle.seek(toOffset: UInt64(section.offset))
            if let data = try fileHandle.read(upToCount: section.length) {
                if let content = String(data: data, encoding: .utf8) {
                    sectionCache[section.id] = content
                    print("[MmapFileReader] getSectionContentFallback: success, id=\(section.id), length=\(content.count)")
                    return content
                }
            }
        } catch {
            print("[MmapFileReader] getSectionContentFallback: error=\(error)")
        }
        return nil
    }
}
