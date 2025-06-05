import IssueDetail from "@/client/components/IssueDetail"

interface Props {
  params: { owner: string; name: string; number: string }
}

export default function IssuePage({ params }: Props) {
  const { owner, name, number } = params

  return (
    <div>
      <IssueDetail owner={owner} repo={name} issueNumber={number} />
    </div>
  )
}
