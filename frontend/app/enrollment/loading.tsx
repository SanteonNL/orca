import { Spinner } from "../../components/spinner";

export default function Loading() {
    return (
        <div className="fixed inset-0 flex items-center justify-center bg-white/80 z-50">
            <Spinner className="h-12 w-12 text-primary" />
        </div>
    );
}
