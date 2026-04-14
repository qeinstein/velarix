import fs from 'fs';
import path from 'path';
import matter from 'gray-matter';

const docsDirectory = path.join(process.cwd(), 'docs');

export interface DocMetadata {
  slug: string;
  title: string;
  description: string;
  order: number;
  section: string;
  sectionOrder: number;
}

export interface DocContent extends DocMetadata {
  content: string;
}

export function getDocsList(): DocMetadata[] {
  if (!fs.existsSync(docsDirectory)) return [];
  const fileNames = fs.readdirSync(docsDirectory);
  const allDocsData = fileNames
    .filter(fileName => fileName.endsWith('.md'))
    .map(fileName => {
      const slug = fileName.replace(/\.md$/, '');
      const fullPath = path.join(docsDirectory, fileName);
      const fileContents = fs.readFileSync(fullPath, 'utf8');
      const matterResult = matter(fileContents);

      return {
        slug,
        title: matterResult.data.title || slug,
        description: matterResult.data.description || '',
        order: matterResult.data.order || 99,
        section: matterResult.data.section || 'General',
        sectionOrder: matterResult.data.sectionOrder || 99,
      };
    });

  return allDocsData.sort((a, b) => {
    if (a.sectionOrder !== b.sectionOrder) return a.sectionOrder - b.sectionOrder;
    if (a.order !== b.order) return a.order - b.order;
    return a.title.localeCompare(b.title);
  });
}

export function getDocBySlug(slug: string): DocContent | null {
  try {
    const fullPath = path.join(docsDirectory, `${slug}.md`);
    const fileContents = fs.readFileSync(fullPath, 'utf8');
    const matterResult = matter(fileContents);

    return {
      slug,
      title: matterResult.data.title || slug,
      description: matterResult.data.description || '',
      order: matterResult.data.order || 99,
      section: matterResult.data.section || 'General',
      sectionOrder: matterResult.data.sectionOrder || 99,
      content: matterResult.content,
    };
  } catch (error) {
    return null;
  }
}

export interface TocHeading {
  level: number;
  text: string;
  id: string;
}

export function extractHeadings(content: string): TocHeading[] {
  const headings: TocHeading[] = [];
  const headingRegex = /^(#{2,3})\s+(.*)$/gm;
  let match;

  while ((match = headingRegex.exec(content)) !== null) {
    const level = match[1].length;
    const text = match[2];
    const id = text.toLowerCase().replace(/[^\w]+/g, '-').replace(/(^-|-$)/g, '');
    headings.push({ level, text, id });
  }

  return headings;
}
