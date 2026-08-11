import { ProjectDetails } from "../../../components/ProjectDetails";

export default async function ProjectRoute({ params }: { params: Promise<{ name: string }> }) {
  const { name } = await params;
  return <ProjectDetails name={decodeURIComponent(name)} />;
}
