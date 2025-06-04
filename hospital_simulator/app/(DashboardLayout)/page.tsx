'use client'
import {redirect, useRouter} from "next/navigation";
import {useEffect} from "react";

const Dashboard = () => {
    useEffect(() => {
        redirect('/patients');
    }, [redirect]);
    return <p></p>;
}

export default Dashboard;
