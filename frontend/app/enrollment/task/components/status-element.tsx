export default function StatusElement  ({ label, value, noUpperCase }: { label: string, value: string, noUpperCase?: boolean | undefined }) {
    return(<>
        <div className={"font-[500]"}>{label}:</div>
        <div className={!noUpperCase ? "first-letter:uppercase" : ""}>{value}</div>
    </>)
}