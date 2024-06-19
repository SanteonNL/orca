import Link from "next/link";
import { styled } from "@mui/material";
import Image from "next/image";

const LinkStyled = styled(Link)(() => ({
  height: "64px",
  width: "219px",
  overflow: "hidden",
  display: "block",
  marginTop: "15px"
}));

const Logo = () => {
  return (
    <LinkStyled href="/">
      <Image src="/images/logos/logo.png" alt="logo" height={64} width={219} priority />
    </LinkStyled>
  );
};

export default Logo;
