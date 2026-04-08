import fs from 'fs';
import path from 'path';
import matter from 'gray-matter';

const docsDirectory = path.join(process.cwd(), 'docs');

export interface DocMetadata {
  slug: string;
  title: string;
  description: string;
  order: number;
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
      };
    });

  return allDocsData.sort((a, b) => a.order - b.order);
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
      content: matterResult.content,
    };
  } catch (error) {
    return null;
  }
}
