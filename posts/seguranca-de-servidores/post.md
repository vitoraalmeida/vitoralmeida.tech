## Motivação

Quando eu estava colocando meu site pessoal no ar, decidi usar um VPS (Virtual Private Server), pois me proporcionaria flexibilidade e customização para atender minhas necessidades. Para servir um simples site estático, pode ser uma escolha ruim, pois existem formas mais baratas e diretas para isso, como o [github pages](https://pages.github.com/). Porém, também gostaria de poder ter um servidor remoto para fim de estudos e também poder servir mais projetos pessoais ao mesmo tempo.

Após tomar a decisão e alugar o servidor, surge a necessidade de protegê-lo contra acessos indesejados. Pesquisando na web sobre as melhores práticas na segurança de servidores, as dicas mais comuns são:

* Alterar a porta padrão do SSH
* Desabilitar o uso de senhas para acesso SSH
* Desabilitar o login por SSH com usuário root
* Habilitar atualizações automáticas
* Usar um firewall

Mas será que tudo isso é necessário e caso não façamos estaremos inseguros?

## Depende...

Como um profissional da área de segurança da informação, aprendi a sempre levar em consideração o contexto do ativo (aquilo que tem valor para uma organização e que deve ser protegido) para determinar a melhor forma de deixá-lo seguro. Segurança absoluta não existe, então devemos sempre tentar fazer o melhor possível, de acordo com as necessidades, com os meios disponíveis, mantendo um bom nível de conveniência.

Fazer uma modelagem de ameaças ajuda tomar a decisão, então devemos nos perguntar pelo menos:

* Contra quem (agente) estamos nos protegendo?
* Com quais ações devemos nos preocupar?
* Quais os objetivos de quem nos ameaça e com qual motivação?
* Quais os meios que esse agente possui para nos prejudicar?
* Quão qualificado é esse agente?
* Quão valioso é o ativo?

Com base nas repostas podemos concluir quais são as ameaças, como mitigá-las, e se as medidas de segurança aplicadas são adequadas ao contexto.

Por exemplo, se identificamos que o agente é alguém muito qualificado e possui todos os meios disponíveis atualmente para realizar ataques (alô NSA), as medidas que devemos tomar para nos proteger devem ser mais robustas que as adotadas contra agentes menos qualificados e menos poderosos (ex.: script kiddies). Se o ativo em questão não for tão importante, a equação também muda, pois também é menos provável que alguém muito qualificado esteja atrás de um recurso menos valioso.

Além disso, a depender da qualificação do adversário, algumas medidas tomadas podem ser inefetivas e é apenas uma questão de tempo até serem superadas. Então, se para adotar tal medida foi necessário montar um esquema complexo de ser implementado e mantido, talvez não valha tanto o esforço, já que sabemos que em algum momento ela vai ser suplantada.

Em resumo, como quase tudo em TI, podemos ligar o "senior mode" e dizer: depende. 

> <cite> palmas lentas </cite> 

## Sempre questione, ainda que seja amplamente aceito como verdade

É obvio que não é para colocar o chapéu de alumínio e começar a conspirar contra toda e qualquer boa prática divulgada sobre qualquer assunto, porém faz bem não assumir toda "boa prática" divulgada nos conteúdos da internet como uma verdade inquestionável.

Dito isso, vamos refletir sobre as recomendações citadas no início do texto.

### Alterar a porta padrão do SSH

A porta 22 é amplamente conhecida como a que é utilizada pelo SSH. Pensando nisso, com o objetivo de atrapalhar a coleta de informações de um possível atacante, recomenda-se que troquemos a porta em que nosso serviço escuta. Supostamente, um atacante executando um `nmap` (utilitário que, dentre outras coisas, escaneia quais portas estão abertas num sistema) buscando pelas portas mais comuns, não veria que temos um SSH rodando no nosso servidor.

No entanto, as portas alternativas utilizadas pela grande maioria das pessoas seguem um certo padrão.

```
$ shodan stats --facets port ssh
Top 10 Results for Facet: port
22             19,811,983
2222              799,310
1080              166,397
10001             154,277
60022             149,733
50022             110,499
50000              83,115
58222              65,517
3389               60,378
1337               55,824
```

O [Shodan](https://www.shodan.io/) é uma ferramenta que mapeia os servidores públicos na Internet e consolida algumas informações sobre eles, como portas abertas, serviços executando em cada porta, qual tipo de dispositivo está em execução etc. Se registrando no site, você tem acesso a uma API Key e, através dela, podemos ter acesso a algumas informações. Podemos ver na saída do comando acima, que, como esperado, a maioria dos serviços SSH estão executando na porta 22. Já a segunda porta mais usada é a 2222, seguida de outras que são mais ou menos fáceis de lembrar.

Podemos ver que para dificultar de fato que um atacante adivinhe em qual porta seu serviço SSH está executando, deveríamos escolher uma porta de forma aleatória. Ainda assim, não existem tantas portas disponíveis (65535) e basta executar o `nmap` habilitando o scan em todas as portas para que o serviço seja descoberto (ex: `nmap -sS -Pn -T5 -p- <ip> `).

A base dessa abordagem é a chamada [**Segurança por Obscuridade**](https://pt.wikipedia.org/wiki/Seguran%C3%A7a_por_obscurantismo), apostando em esconder informações confiando que é o suficiente para manter algo seguro. 

> <cite>Pessoas desonestas são muito profissionais e já sabem muito mais do que poderíamos ensiná-los</cite>
>
> -- <cite>Alfred Charles Hobbs</cite>

Além de não ser uma medida efetiva, alterar a porta pela qual você acessa seu servidor SSH pode te confundir caso você trabalhe sozinho e tenha uma memória ruim ou caso trabalhe numa equipe maior. Onde você vai documentar qual porta está sendo usada? As pessoas que trabalham com você sabem dessa alteração e dessa documentação? Claro que nesse simples caso de uma porta SSH não é tão complicado de resolver, mas quando tratamos de serviços e ativos mais críticos, com mais pessoas envolvidas, segurança por obscuridade acaba gerando complexidades, dificuldades de entendimento pelos membros de um time e, além de tudo, não funciona.

### Desabilitar o uso de senhas para acesso SSH

O arquivo de configuração do servidor SSH (`/etc/ssh/sshd_config`) traz o seguinte:

> <cite>...</cite>
>
> <cite>\# To disable tunneled clear text passwords, change to no here! </cite>
> <cite>PasswordAuthentication yes<\cite>
>
> <cite>...</cite>

Ou seja, aparentemente, a senha que você envia durante a conexão com SSH é transmitida em texto claro dentro do "túnel" até chegar no servidor remoto. Então isso quer dizer que a sua senha está exposta para qualquer um que intercepte a conexão possa ver? Não! Pois, a conexão com o servidor SSH acontece utilizando um par de chaves criptográficas para mascarar os dados que tráfegam no estabelecimento da conexão com o servidor remoto. É a mesma coisa que acontece quando nos autenticamos na maioria dos sites que utilizam HTTPS. A nossa senha é encapsulada numa conexão SSL que trafega criptografada até chegar no servidor.

Não é perfeitamente seguro utilizar senhas ao se conectar por SSH, como a própria [documentação](https://datatracker.ietf.org/doc/html/rfc4251#section-9.4.5) afirma:

>  <cite>The password mechanism, as specified in the authentication protocol, assumes that the server has not been compromised.  If the server has been compromised, using password authentication will reveal a valid username/password combination to the attacker, which may lead to further compromises. </cite>

>  <cite>This vulnerability can be mitigated by using an alternative form of authentication.  For example, public key authentication makes no assumptions about security on the server. </cite>

O mecanismo de autenticação por senha assume que o servidor do SSH não foi comprometido, mas, nesse caso, já temos um problema e não há muito mais o que fazer (haha xD). A documentação afirma que podemos mitigar isso usando autenticação com chaves, mas...

>  <cite>The use of public key authentication assumes that the client host has not been compromised.  It also assumes that the private key of the server host has not been compromised. </cite>

>  <cite>This risk can be mitigated by the use of passphrases on private keys; however, this is not an enforceable policy.  The use of smartcards, or other technology to make passphrases an enforceable policy is suggested.</cite>

A mesma documentação do protocolo, agora na seção sobre a [autenticação com chaves](https://datatracker.ietf.org/doc/html/rfc4251#section-9.4.4), traz que o método também não é perfeito, pois assume que o dispositivo cliente também não foi comprometido. Ou seja, não é o uso de senhas nessa conexão que é especialmente inseguro, mas depende de um conjunto de fatores.

Usar senhas ainda é algo complicado, pois depende que sempre usemos senhas fortes e que tenhamos como armazená-las em lugares seguros. Então, de fato, pode ser que seja bom desabilitar a autenticação por senha e usar chaves, mas não é porque é inseguro em todo caso.

### Desabilitar o login por SSH com usuário root


