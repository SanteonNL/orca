import Link from "next/link";
import { styled } from "@mui/material";
import Image from "next/image";

const LinkStyled = styled(Link)(() => ({
  height: "153px",
  width: "219px",
  overflow: "hidden",
  display: "block",
  marginTop: "15px"
}));

const Logo = () => {
  return (
    <LinkStyled href="/">
      <Image src={`${process.env.NEXT_PUBLIC_BASE_PATH || ""}/images/logos/logo.svg`} alt="logo" height={153} width={219} priority />
    </LinkStyled>
  );
};

export default Logo;
